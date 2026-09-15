package app

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend/internal/config"
	"backend/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Application struct {
	Config *config.Config
	DB     *pgxpool.Pool
	Models models.Models
}

func (app *Application) Run(r http.Handler) error {
	srv := &http.Server{
		Addr:         app.Config.Addr,
		Handler:      r,
		WriteTimeout: app.Config.WriteTimeout,
		ReadTimeout:  app.Config.ReadTimeout,
		IdleTimeout:  app.Config.IdleTimeout,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)
		log.Printf("🚀 server is running at %s", app.Config.Addr)
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err

	case sig := <-shutdown:
		log.Printf("📦 received shutdown signal: %v", sig)

		// Use configurable timeout
		shutdownTimeout := app.Config.ShutdownTimeout
		if shutdownTimeout == 0 {
			shutdownTimeout = 10 * time.Second // fallback default
		}

		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		log.Printf("⏳ waiting up to %v for graceful shutdown...", shutdownTimeout)

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("❌ graceful shutdown failed: %v", err)
			return err
		}

		app.DB.Close()
		log.Println("✅ server closed gracefully")
	}

	return nil
}
