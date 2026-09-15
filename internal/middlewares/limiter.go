package middlewares

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"backend/internal/utils"
)

type client struct {
	tokens    int
	lastRefil time.Time
}

type RateLimiter struct {
	mu         sync.Mutex
	clients    map[string]*client
	rate       int
	interval   time.Duration
	trustProxy bool
}

// NewRateLimiter creates a limiter that keys on the client IP.
// If trustProxy is true, the first value in X-Forwarded-For is used
// instead of RemoteAddr. Only enable trustProxy when running behind
// a reverse proxy you control — otherwise clients can spoof the header.
func NewRateLimiter(rate int, interval time.Duration, trustProxy ...bool) *RateLimiter {
	tp := false
	if len(trustProxy) > 0 {
		tp = trustProxy[0]
	}

	rl := &RateLimiter{
		clients:    make(map[string]*client),
		rate:       rate,
		interval:   interval,
		trustProxy: tp,
	}

	rl.Cleanup()
	return rl
}

func (rl *RateLimiter) clientIP(r *http.Request) string {
	if rl.trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if idx := strings.IndexByte(xff, ','); idx != -1 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func (rl *RateLimiter) LimitRate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rl.clientIP(r)

		rl.mu.Lock()
		defer rl.mu.Unlock()

		c, exists := rl.clients[ip]
		if !exists {
			rl.clients[ip] = &client{
				tokens:    rl.rate - 1,
				lastRefil: time.Now(),
			}
			next.ServeHTTP(w, r)
			return
		}

		now := time.Now()
		if now.Sub(c.lastRefil) > rl.interval {
			c.tokens = rl.rate
			c.lastRefil = now
		}

		if c.tokens <= 0 {
			utils.ErrorJson(w, r, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		c.tokens--
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) Cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			rl.mu.Lock()
			for ip, c := range rl.clients {
				if time.Since(c.lastRefil) > rl.interval*2 {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
}
