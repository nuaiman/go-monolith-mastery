package bootstrap

import (
	"context"
	"log"

	"backend/internal/app"
	"backend/internal/constants"
	"backend/internal/models"
	"backend/internal/utils"
)

// SeedSuperAdmin creates the initial superadmin account if none exists.
// It reads credentials from config and is idempotent — safe to call on
// every startup.
//
// If SUPER_ADMIN_EMAIL is empty, this is a no-op.
func SeedSuperAdmin(app *app.Application) error {
	email := app.Config.SuperAdminEmail
	if email == "" {
		log.Println("ℹ️  super admin seeding skipped (SUPER_ADMIN_EMAIL not set)")
		return nil
	}

	ctx := context.Background()

	// Already seeded?
	existing, err := app.Models.User.GetByEmail(ctx, email)
	if err != nil {
		return err
	}
	if existing != nil {
		log.Printf("✅ super admin already exists: %s", email)
		return nil
	}

	// Require password
	if app.Config.SuperAdminPassword == "" {
		log.Println("⚠️  SUPER_ADMIN_PASSWORD not set — skipping super admin seed")
		return nil
	}
	if len(app.Config.SuperAdminPassword) < 8 {
		log.Println("⚠️  SUPER_ADMIN_PASSWORD must be at least 8 characters — skipping seed")
		return nil
	}

	hashedPassword, err := utils.HashPassword(app.Config.SuperAdminPassword)
	if err != nil {
		return err
	}

	name := app.Config.SuperAdminName
	if name == "" {
		name = "Super Admin"
	}

	user := &models.User{
		Role:     constants.RoleSuperAdmin,
		IsActive: true,
		Email:    email,
		Password: &hashedPassword,
		Name:     &name,
	}

	if err := app.Models.User.Insert(ctx, user); err != nil {
		return err
	}

	log.Printf("✅ super admin created: id=%s email=%s", user.ID, user.Email)
	return nil
}
