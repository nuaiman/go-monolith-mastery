package config

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	Addr string
	Env  string

	// Graceful Shutdown
	ShutdownTimeout time.Duration

	// Database
	PGURL string

	// Security Tokens
	JWTKey          string
	JWTExpiryHours  int
	RefreshTokenAge time.Duration

	// Bootstrap Super Admin
	SuperAdminEmail    string
	SuperAdminPassword string
	SuperAdminName     string

	// Server Timeouts
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// Rate Limiting
	RateLimit    int
	RateInterval time.Duration

	// Trust X-Forwarded-For for client IP
	TrustProxy bool

	// File Paths
	PublicDir string
	BucketDir string

	// Uploads
	MaxUploadBytes int64

	// CORS
	CORSAllowedOrigins []string
}

// MustLoadConfig loads the .env file (if present), reads configuration
// from environment variables, validates, and returns the config.
//
// Load order (highest priority first):
//  1. Real process env (docker, CI, shell export)
//  2. .env file
//  3. Hardcoded fallback in load()
//
// The .env path is resolved from (in order):
//   - -config flag
//   - CONFIG_PATH environment variable
//   - "config/config.env" (default)
func MustLoadConfig() *Config {
	envPath := resolveEnvPath()

	// godotenv.Load does NOT override existing process env vars.
	// That's exactly what we want: real env beats the .env file.
	if err := godotenv.Load(envPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("cannot load env file %s: %v", envPath, err)
	}

	cfg, err := load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	log.Printf("✅ config loaded (env: %s)", cfg.Env)
	return cfg
}

func resolveEnvPath() string {
	var envPath string
	flag.StringVar(&envPath, "config", "", "path to config file")
	flag.Parse()

	if envPath != "" {
		return envPath
	}
	if envPath = os.Getenv("CONFIG_PATH"); envPath != "" {
		return envPath
	}
	return "config/config.env"
}

func load() (*Config, error) {
	cfg := &Config{
		// --- Required (empty = error, caught by Validate) ---
		PGURL:  os.Getenv("PG_URL"),
		JWTKey: os.Getenv("JWT_KEY"),

		// --- Optional with sensible defaults ---
		Addr:            getString("ADDR", ":8080"),
		Env:             getString("ENV", "development"),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),

		JWTExpiryHours:  getInt("JWT_EXPIRY_HOURS", 24),
		RefreshTokenAge: getDuration("REFRESH_TOKEN_AGE", 720*time.Hour),

		SuperAdminEmail:    os.Getenv("SUPER_ADMIN_EMAIL"),
		SuperAdminPassword: os.Getenv("SUPER_ADMIN_PASSWORD"),
		SuperAdminName:     getString("SUPER_ADMIN_NAME", "Super Admin"),

		ReadTimeout:  getDuration("READ_TIMEOUT", 10*time.Second),
		WriteTimeout: getDuration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:  getDuration("IDLE_TIMEOUT", time.Minute),

		RateLimit:    getInt("RATE_LIMIT", 100),
		RateInterval: getDuration("RATE_INTERVAL", time.Minute),
		TrustProxy:   getBool("TRUST_PROXY", false),

		PublicDir: getString("PUBLIC_DIR", "./public"),
		BucketDir: getString("BUCKET_DIR", "./bucket"),

		MaxUploadBytes: getInt64("MAX_UPLOAD_BYTES", 5*1024*1024),

		CORSAllowedOrigins: getCSV("CORS_ALLOWED_ORIGINS"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ============================================================
// ENV HELPERS
// ============================================================

func getString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("%s must be an integer, got %q", key, v)
	}
	return n
}

func getInt64(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		log.Fatalf("%s must be an integer, got %q", key, v)
	}
	return n
}

func getBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		log.Fatalf("%s must be a boolean, got %q", key, v)
	}
	return b
}

func getDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Fatalf("%s must be a duration (e.g. 10s, 1m, 720h), got %q", key, v)
	}
	return d
}

// getCSV reads a comma-separated list, trimming whitespace from each
// entry. Empty env var returns nil.
func getCSV(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// ============================================================
// VALIDATION
// ============================================================

func (cfg *Config) Validate() error {
	if _, err := net.ResolveTCPAddr("tcp", cfg.Addr); err != nil {
		return fmt.Errorf("invalid ADDR: %w", err)
	}
	if cfg.PGURL == "" {
		return errors.New("PG_URL is required but not set")
	}
	if cfg.JWTKey == "" {
		return errors.New("JWT_KEY is required but not set")
	}
	if cfg.JWTExpiryHours <= 0 {
		return errors.New("JWT_EXPIRY_HOURS must be greater than 0")
	}
	if cfg.RefreshTokenAge <= 0 {
		return errors.New("REFRESH_TOKEN_AGE must be greater than 0")
	}
	if cfg.RateLimit <= 0 {
		return errors.New("RATE_LIMIT must be greater than 0")
	}
	if cfg.RateInterval <= 0 {
		return errors.New("RATE_INTERVAL must be greater than 0")
	}
	if cfg.ShutdownTimeout <= 0 {
		return errors.New("SHUTDOWN_TIMEOUT must be greater than 0")
	}
	if cfg.MaxUploadBytes <= 0 {
		return errors.New("MAX_UPLOAD_BYTES must be greater than 0")
	}
	if cfg.SuperAdminEmail != "" && len(cfg.SuperAdminPassword) < 8 {
		return errors.New("SUPER_ADMIN_PASSWORD must be at least 8 characters when SUPER_ADMIN_EMAIL is set")
	}
	return nil
}
