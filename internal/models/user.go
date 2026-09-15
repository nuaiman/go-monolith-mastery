package models

import (
	"context"
	"errors"
	"time"

	"backend/internal/constants"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Role     string `json:"role"`
	IsActive bool   `json:"is_active"`
	Email    string `json:"email"`

	Password       *string    `json:"-"`
	RefreshToken   *string    `json:"-"`
	RefreshTokenAt *time.Time `json:"-"`

	Name     *string `json:"name,omitempty"`
	ImageURL *string `json:"image_url,omitempty"`
}

type UserModel struct {
	DB *pgxpool.Pool
}

const userSelectColumns = `
	id, created_at, updated_at,
	role, is_active, email, password,
	refresh_token, refresh_token_at,
	name, image_url
`

func scanUser(row pgx.Row) (*User, error) {
	u := &User{}
	var pgID pgtype.UUID
	err := row.Scan(
		&pgID, &u.CreatedAt, &u.UpdatedAt,
		&u.Role, &u.IsActive, &u.Email, &u.Password,
		&u.RefreshToken, &u.RefreshTokenAt,
		&u.Name, &u.ImageURL,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	u.ID = pgID.String()
	return u, nil
}

func scanUsers(rows pgx.Rows) ([]*User, error) {
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		var pgID pgtype.UUID
		err := rows.Scan(
			&pgID, &u.CreatedAt, &u.UpdatedAt,
			&u.Role, &u.IsActive, &u.Email, &u.Password,
			&u.RefreshToken, &u.RefreshTokenAt,
			&u.Name, &u.ImageURL,
		)
		if err != nil {
			return nil, err
		}
		u.ID = pgID.String()
		users = append(users, u)
	}
	return users, rows.Err()
}

func (um *UserModel) Insert(ctx context.Context, user *User) error {
	if !constants.IsValidRole(user.Role) {
		return errors.New("invalid role")
	}

	const query = `
		INSERT INTO users (role, is_active, email, password, name, image_url)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`
	var pgID pgtype.UUID
	err := um.DB.QueryRow(ctx, query,
		user.Role, user.IsActive, user.Email, user.Password,
		user.Name, user.ImageURL,
	).Scan(&pgID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return err
	}
	user.ID = pgID.String()
	return nil
}

func (um *UserModel) GetByID(ctx context.Context, id string) (*User, error) {
	query := `SELECT ` + userSelectColumns + ` FROM users WHERE id = $1`
	return scanUser(um.DB.QueryRow(ctx, query, id))
}

func (um *UserModel) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `SELECT ` + userSelectColumns + ` FROM users WHERE email = $1`
	return scanUser(um.DB.QueryRow(ctx, query, email))
}

func (um *UserModel) GetByRefreshToken(ctx context.Context, token string) (*User, error) {
	query := `SELECT ` + userSelectColumns + ` FROM users WHERE refresh_token = $1`
	return scanUser(um.DB.QueryRow(ctx, query, token))
}

func (um *UserModel) GetAll(ctx context.Context) ([]*User, error) {
	query := `SELECT ` + userSelectColumns + ` FROM users ORDER BY created_at DESC`
	rows, err := um.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	return scanUsers(rows)
}

func (um *UserModel) GetByRole(ctx context.Context, role string) ([]*User, error) {
	if !constants.IsValidRole(role) {
		return nil, errors.New("invalid role")
	}
	query := `SELECT ` + userSelectColumns + ` FROM users WHERE role = $1 ORDER BY created_at DESC`
	rows, err := um.DB.Query(ctx, query, role)
	if err != nil {
		return nil, err
	}
	return scanUsers(rows)
}

func (um *UserModel) UpdateProfile(ctx context.Context, user *User) error {
	const query = `
		UPDATE users SET
			name = $1,
			image_url = $2,
			updated_at = NOW()
		WHERE id = $3
	`
	result, err := um.DB.Exec(ctx, query, user.Name, user.ImageURL, user.ID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (um *UserModel) UpdateByAdmin(ctx context.Context, user *User) error {
	const query = `
		UPDATE users SET
			is_active = $1,
			email = $2,
			name = $3,
			image_url = $4,
			updated_at = NOW()
		WHERE id = $5
	`
	result, err := um.DB.Exec(ctx, query,
		user.IsActive, user.Email, user.Name, user.ImageURL, user.ID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (um *UserModel) SetRole(ctx context.Context, id string, role string) error {
	if !constants.IsValidRole(role) {
		return errors.New("invalid role")
	}
	const query = `UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2`
	result, err := um.DB.Exec(ctx, query, role, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (um *UserModel) SetImageURL(ctx context.Context, id string, url *string) error {
	const query = `UPDATE users SET image_url = $1, updated_at = NOW() WHERE id = $2`
	result, err := um.DB.Exec(ctx, query, url, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (um *UserModel) UpdatePassword(ctx context.Context, id string, hashedPassword string) error {
	const query = `UPDATE users SET password = $1, updated_at = NOW() WHERE id = $2`
	result, err := um.DB.Exec(ctx, query, hashedPassword, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (um *UserModel) UpdateRefreshToken(ctx context.Context, id string, token *string) error {
	const query = `
		UPDATE users SET
			refresh_token = $1,
			refresh_token_at = NOW()
		WHERE id = $2
	`
	result, err := um.DB.Exec(ctx, query, token, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (um *UserModel) SetActive(ctx context.Context, id string, active bool) error {
	const query = `UPDATE users SET is_active = $1, updated_at = NOW() WHERE id = $2`
	result, err := um.DB.Exec(ctx, query, active, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (um *UserModel) Delete(ctx context.Context, id string) error {
	const query = `DELETE FROM users WHERE id = $1`
	result, err := um.DB.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (um *UserModel) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	var exists bool
	err := um.DB.QueryRow(ctx, query, email).Scan(&exists)
	return exists, err
}

func (um *UserModel) Count(ctx context.Context) (int64, error) {
	const query = `SELECT COUNT(*) FROM users`
	var count int64
	err := um.DB.QueryRow(ctx, query).Scan(&count)
	return count, err
}

func (um *UserModel) CountByRole(ctx context.Context, role string) (int64, error) {
	if !constants.IsValidRole(role) {
		return 0, errors.New("invalid role")
	}
	const query = `SELECT COUNT(*) FROM users WHERE role = $1`
	var count int64
	err := um.DB.QueryRow(ctx, query, role).Scan(&count)
	return count, err
}
