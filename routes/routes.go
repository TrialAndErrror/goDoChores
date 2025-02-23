package routes

import "fmt"

var Routes = map[string]string{
	"login":           "/login",
	"home":            "/",
	"choresList":      "/chores/",
	"choresCreate":    "/chores/new",
	"choresDetail":    "/chores/%d",
	"remindersList":   "/reminders/",
	"remindersCreate": "/reminders/new",
	"remindersDetail": "/reminders/%d",
}

func URLFor(route string, args ...any) string {
	return fmt.Sprintf(Routes[route], args...)
}
