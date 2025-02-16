package main

import (
	"goDoChores/models"
	"goDoChores/routes"
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
	r.Get(routes.URLFor("home"), views.Home)
	r.Get(routes.URLFor("choresList"), views.ChoresList)
	log.Fatal(http.ListenAndServe(":3000", r))
}
