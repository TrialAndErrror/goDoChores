package main

import (
	"goDoChores/models"
	"goDoChores/views"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	db, dbConnectErr := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if dbConnectErr != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	migrateErr := db.AutoMigrate(&models.Chore{})
	if migrateErr != nil {
		panic("failed to migrate database")
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", views.Home)
	r.Get("/chores/", views.ChoresList)
	r.Get("/chores/new", views.ChoresCreateGet)
	r.Post("/chores/new", views.ChoresCreatePost)
	r.Get("/chores/{choreID}", views.ChoresDetail)
	log.Fatal(http.ListenAndServe(":3000", r))
}
