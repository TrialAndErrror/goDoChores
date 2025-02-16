package routes

import "fmt"

var Routes = map[string]string{
	"home":         "/",
	"choresList":   "/chores/",
	"choresCreate": "/chores/new",
	"choresDetail": "/chores/%d",
}

func URLFor(route string, args ...any) string {
	return fmt.Sprintf(Routes[route], args...)
}
