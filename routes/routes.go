package routes

import "fmt"

var Routes = map[string]string{
	"login":           "/login",
	"home":            "/",
	"choresList":      "/chores/",
	"choresCreate":    "/chores/new",
	"choresDetail":    "/chores/%d/",
	"choresEdit":      "/chores/%d/edit",
	"remindersList":   "/reminders/",
	"remindersCreate": "/reminders/new",
	"remindersDetail": "/reminders/%d/",
	"remindersEdit":   "/reminders/%d/edit",
	"tasksList":       "/tasks/",
	"tasksCreate":     "/tasks/new",
	"tasksDetail":     "/tasks/%d/",
	"tasksEdit":       "/tasks/%d/edit",
	"logout":          "/logout",
}

func URLFor(route string, args ...any) string {
	return fmt.Sprintf(Routes[route], args...)
}
