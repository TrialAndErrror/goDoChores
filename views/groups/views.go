package groups

import "github.com/go-chi/chi/v5"

func GroupsRouter() chi.Router {
	r := chi.NewRouter()

	r.Get("/", Page)
	r.Post("/new", Create)
	r.Post("/rename", Rename)
	r.Post("/invite", Invite)
	r.Post("/tags", SaveTags)
	r.Post("/tags/new", CreateTag)
	r.Post("/leave", Leave)
	r.Post("/delete", Delete)
	r.Post("/invites/{inviteID}/{answer}", AnswerInvite)

	return r
}
