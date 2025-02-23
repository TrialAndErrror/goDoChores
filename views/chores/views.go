package chores

import "github.com/go-chi/chi/v5"

func ChoresRouter() chi.Router {
	r := chi.NewRouter()

	r.Get("/", List)
	r.Get("/new", CreateGet)
	r.Post("/new", CreatePost)
	r.Get("/{choreID}/", DetailGet)
	r.Post("/{choreID}/", DetailPost)

	return r
}
