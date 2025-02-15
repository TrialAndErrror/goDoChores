package main

import (
	"goDoChores/models"
	"goDoChores/routes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {

	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	db.AutoMigrate(&models.Chore{})

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", routes.Home)
	r.Get("/chores", routes.ChoresList)
	http.ListenAndServe(":3000", r)
}
