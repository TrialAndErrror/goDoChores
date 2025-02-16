package routes

import "fmt"

var Routes = map[string]string{
	"home":       "/",
	"choresList": "/chores",
}

func URLFor(route string, args ...any) string {
	return fmt.Sprintf(Routes[route], args...)
}
