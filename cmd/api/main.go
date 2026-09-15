package main

import (
	"log"
	"path/filepath"

	"backend/internal/app"
	"backend/internal/bootstrap"
	"backend/internal/config"
	"backend/internal/db"
	"backend/internal/handlers"
	"backend/internal/models"
	"backend/internal/router"
)

// @title           Backend API
// @version         1.0
// @description     REST API for posts, users, and image uploads.
// @description
// @description     Auth: JWT access token (24h) + rotating refresh token (30d).
// @description     Roles: user, admin, superadmin. Admin+ can moderate users.
// @description     All responses include a request_id for log correlation.
// @contact.name    API Support
// @contact.email   support@backend.local
// @license.name    MIT
// @license.url     https://opensource.org/licenses/MIT
// @host            localhost:8080
// @BasePath        /api
// @schemes         http https
// @tag.name         healthz
// @tag.description  Liveness probe
// @tag.name         auth
// @tag.description  Authentication and session management
// @tag.name         users
// @tag.description  User profiles and administrative operations
// @tag.name         posts
// @tag.description  Post CRUD — public reads, auth-required writes
// @tag.name         uploads
// @tag.description  Image uploads — one image per user, one per post
// @tag.name         admin
// @tag.description  Admin operations (admin or superadmin)
// @tag.name         super
// @tag.description  Superadmin-only operations
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     Enter: Bearer {token}
func main() {
	// Load config
	cfg := config.MustLoadConfig()

	// Connect to database with retry logic
	log.Println("🔄 connecting to database with retry...")
	dbPool, err := db.Connect(cfg.PGURL, db.DefaultRetryConfig())
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.ClosePool(dbPool)

	// Run migrations
	schemaPath := filepath.Join("internal", "db", "migrations", "001_schema.sql")
	if err := db.RunMigrations(dbPool, schemaPath); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// Initialize application
	a := &app.Application{
		Config: cfg,
		DB:     dbPool,
		Models: models.NewModels(dbPool),
	}

	// Seed super admin (idempotent)
	if err := bootstrap.SeedSuperAdmin(a); err != nil {
		log.Fatalf("failed to seed super admin: %v", err)
	}

	// Setup handlers and router
	h := handlers.New(a)
	r := router.RegisterRouter(a, h)

	// Tell the operator where to find things.
	log.Printf("📖 Swagger UI: http://localhost%s/swagger/index.html", cfg.Addr)

	// Start server
	if err := a.Run(r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// psql -U postgres -p 5432
// DROP DATABASE backend;
// CREATE DATABASE backend;

// ngrok http --scheme=https 8080

// swag init -g cmd/api/main.go --parseInternal -o docs

// go get -u ./...
// go mod tidy
