// internal/db/postgres.go
package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RetryConfig holds configuration for connection retries
type RetryConfig struct {
	MaxAttempts       int
	InitialDelay      time.Duration
	MaxDelay          time.Duration
	BackoffMultiplier float64
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:       10,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// Connect creates a connection pool with retry logic
func Connect(connectionString string, retryConfig RetryConfig) (*pgxpool.Pool, error) {
	var lastErr error
	delay := retryConfig.InitialDelay

	for attempt := 1; attempt <= retryConfig.MaxAttempts; attempt++ {
		log.Printf("🔄 connecting to PostgreSQL (attempt %d/%d)...", attempt, retryConfig.MaxAttempts)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		pool, err := connectWithConfig(ctx, connectionString)
		cancel()

		if err == nil {
			log.Printf("✅ connected to Postgres successfully")
			return pool, nil
		}

		lastErr = err
		log.Printf("❌ attempt %d failed: %v", attempt, err)

		if attempt == retryConfig.MaxAttempts {
			break
		}

		nextDelay := time.Duration(float64(delay) * retryConfig.BackoffMultiplier)
		if nextDelay > retryConfig.MaxDelay {
			nextDelay = retryConfig.MaxDelay
		}

		log.Printf("⏳ waiting %v before next attempt...", delay)
		time.Sleep(delay)
		delay = nextDelay
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", retryConfig.MaxAttempts, lastErr)
}

// connectWithConfig creates a pool with the given context
func connectWithConfig(ctx context.Context, connectionString string) (*pgxpool.Pool, error) {
	pgPoolCfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, err
	}

	pgPoolCfg.MaxConns = 25
	pgPoolCfg.MinConns = 5
	pgPoolCfg.MaxConnLifetime = time.Hour
	pgPoolCfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, pgPoolCfg)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

// RunMigrations executes the schema file if the database hasn't been
// initialized yet.
//
// The check is intentionally simple: if `users` exists, assume the
// schema is present. If you need to change the schema during
// development, drop and recreate the database:
//
//	DROP DATABASE backend;
//	CREATE DATABASE backend;
//
// When you outgrow this, replace with a proper migration tool
// (goose, golang-migrate, or a hand-rolled schema_migrations table).
func RunMigrations(pool *pgxpool.Pool, schemaPath string) error {
	ctx := context.Background()

	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables WHERE table_name = 'users'
		)
	`).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		log.Println("✅ schema already exists, skipping migrations")
		return nil
	}

	log.Println("📦 running database migrations...")

	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}

	// Execute the entire schema as a single statement.
	// This handles semicolons inside CHECK constraints properly.
	if _, err := pool.Exec(ctx, string(schemaBytes)); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	log.Println("✅ migrations completed successfully")
	return nil
}

// ClosePool safely closes the connection pool
func ClosePool(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
		log.Println("✅ database connection pool closed")
	}
}
