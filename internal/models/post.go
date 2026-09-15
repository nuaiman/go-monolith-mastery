package models

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Post struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Title    string  `json:"title"`
	Body     string  `json:"body"`
	ImageURL *string `json:"image_url,omitempty"`
}

type PostModel struct {
	DB *pgxpool.Pool
}

const postSelectColumns = `
	id, user_id, created_at, updated_at,
	title, body, image_url
`

func scanPost(row pgx.Row) (*Post, error) {
	p := &Post{}
	var pgID, pgUserID pgtype.UUID
	err := row.Scan(
		&pgID, &pgUserID, &p.CreatedAt, &p.UpdatedAt,
		&p.Title, &p.Body, &p.ImageURL,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	p.ID = pgID.String()
	p.UserID = pgUserID.String()
	return p, nil
}

func scanPosts(rows pgx.Rows) ([]*Post, error) {
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		p := &Post{}
		var pgID, pgUserID pgtype.UUID
		err := rows.Scan(
			&pgID, &pgUserID, &p.CreatedAt, &p.UpdatedAt,
			&p.Title, &p.Body, &p.ImageURL,
		)
		if err != nil {
			return nil, err
		}
		p.ID = pgID.String()
		p.UserID = pgUserID.String()
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (pm *PostModel) Insert(ctx context.Context, post *Post) error {
	const query = `
		INSERT INTO posts (user_id, title, body, image_url)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`
	var pgID pgtype.UUID
	err := pm.DB.QueryRow(ctx, query,
		post.UserID, post.Title, post.Body, post.ImageURL,
	).Scan(&pgID, &post.CreatedAt, &post.UpdatedAt)
	if err != nil {
		return err
	}
	post.ID = pgID.String()
	return nil
}

func (pm *PostModel) GetByID(ctx context.Context, id string) (*Post, error) {
	query := `SELECT ` + postSelectColumns + ` FROM posts WHERE id = $1`
	return scanPost(pm.DB.QueryRow(ctx, query, id))
}

func (pm *PostModel) List(ctx context.Context, limit, offset int) ([]*Post, error) {
	const query = `
		SELECT ` + postSelectColumns + `
		FROM posts
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := pm.DB.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	return scanPosts(rows)
}

func (pm *PostModel) ListByUser(ctx context.Context, userID string, limit, offset int) ([]*Post, error) {
	const query = `
		SELECT ` + postSelectColumns + `
		FROM posts
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := pm.DB.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	return scanPosts(rows)
}

func (pm *PostModel) CountByUser(ctx context.Context, userID string) (int64, error) {
	const query = `SELECT COUNT(*) FROM posts WHERE user_id = $1`
	var count int64
	err := pm.DB.QueryRow(ctx, query, userID).Scan(&count)
	return count, err
}

func (pm *PostModel) Update(ctx context.Context, post *Post) error {
	const query = `
		UPDATE posts SET
			title = $1,
			body = $2,
			image_url = $3,
			updated_at = NOW()
		WHERE id = $4 AND user_id = $5
	`
	result, err := pm.DB.Exec(ctx, query,
		post.Title, post.Body, post.ImageURL,
		post.ID, post.UserID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("post not found or not owned by user")
	}
	return nil
}

func (pm *PostModel) UpdateAny(ctx context.Context, post *Post) error {
	const query = `
		UPDATE posts SET
			title = $1,
			body = $2,
			image_url = $3,
			updated_at = NOW()
		WHERE id = $4
	`
	result, err := pm.DB.Exec(ctx, query,
		post.Title, post.Body, post.ImageURL, post.ID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("post not found")
	}
	return nil
}

func (pm *PostModel) SetImageURL(ctx context.Context, id, userID string, url *string) error {
	const query = `
		UPDATE posts SET image_url = $1, updated_at = NOW()
		WHERE id = $2 AND user_id = $3
	`
	result, err := pm.DB.Exec(ctx, query, url, id, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("post not found or not owned by user")
	}
	return nil
}

func (pm *PostModel) SetImageURLAny(ctx context.Context, id string, url *string) error {
	const query = `
		UPDATE posts SET image_url = $1, updated_at = NOW()
		WHERE id = $2
	`
	result, err := pm.DB.Exec(ctx, query, url, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("post not found")
	}
	return nil
}

func (pm *PostModel) Delete(ctx context.Context, id, userID string) error {
	const query = `DELETE FROM posts WHERE id = $1 AND user_id = $2`
	result, err := pm.DB.Exec(ctx, query, id, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("post not found or not owned by user")
	}
	return nil
}

func (pm *PostModel) DeleteAny(ctx context.Context, id string) error {
	const query = `DELETE FROM posts WHERE id = $1`
	result, err := pm.DB.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("post not found")
	}
	return nil
}
