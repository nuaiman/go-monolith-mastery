package handlers

import "backend/internal/app"

type Handler struct {
	app *app.Application
}

func New(app *app.Application) *Handler {
	return &Handler{app: app}
}
