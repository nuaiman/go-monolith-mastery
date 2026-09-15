package models

import "github.com/jackc/pgx/v5/pgxpool"

// Models is the single entry point for all database access.
type Models struct {
	User UserModel
	Post PostModel
}

func NewModels(db *pgxpool.Pool) Models {
	return Models{
		User: UserModel{DB: db},
		Post: PostModel{DB: db},
	}
}
