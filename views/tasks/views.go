package tasks

import "github.com/go-chi/chi/v5"

func TasksRouter() chi.Router {
	r := chi.NewRouter()

	r.Get("/", List)
	r.Get("/new", CreateGet)
	r.Post("/new", CreatePost)
	r.Get("/{taskID}/", DetailGet)
	r.Post("/{taskID}/", DetailPost)
	r.Get("/{taskID}/edit", EditGet)
	r.Post("/{taskID}/edit", EditPost)

	return r
}
