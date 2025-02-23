package home

import (
	"github.com/go-chi/chi/v5"
)

func HomeRouter() chi.Router {
	homeRouter := chi.NewRouter()

	homeRouter.Get("/", Home)
	homeRouter.Post("/", HomePost)

	return homeRouter
}
