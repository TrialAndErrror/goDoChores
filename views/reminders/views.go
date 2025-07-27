package reminders

import "github.com/go-chi/chi/v5"

func RemindersRouter() chi.Router {
	r := chi.NewRouter()
	r.Get("/", List)
	r.Get("/new", CreateGet)
	r.Post("/new", CreatePost)
	r.Get("/{reminderID}/", DetailGet)
	r.Post("/{reminderID}/", DetailPost)
	r.Get("/{reminderID}/edit", EditGet)
	r.Post("/{reminderID}/edit", EditPost)
	return r
}
