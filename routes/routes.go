package routes

import "fmt"

var routes = map[string]string{
	"login":              "/login",
	"home":               "/",
	"choresList":         "/chores/",
	"choresCreate":       "/chores/new",
	"choresDetail":       "/chores/%d/",
	"choresEdit":         "/chores/%d/edit",
	"remindersList":      "/reminders/",
	"remindersCreate":    "/reminders/new",
	"remindersDetail":    "/reminders/%d/",
	"remindersEdit":      "/reminders/%d/edit",
	"tasksList":          "/tasks/",
	"tasksCreate":        "/tasks/new",
	"tasksDetail":        "/tasks/%d/",
	"tasksEdit":          "/tasks/%d/edit",
	"groups":             "/groups/",
	"groupsNew":          "/groups/new",
	"groupsRename":       "/groups/rename",
	"groupsInvite":       "/groups/invite",
	"groupsLeave":        "/groups/leave",
	"groupsDelete":       "/groups/delete",
	"groupsInviteAnswer": "/groups/invites/%d/%s",
	"groupsTags":         "/groups/tags",
	"groupsTagNew":       "/groups/tags/new",
	"logout":             "/logout",
}

func URLFor(route string, args ...any) string {
	pattern, ok := routes[route]
	if !ok {
		panic("routes: unknown route " + route)
	}
	return fmt.Sprintf(pattern, args...)
}
