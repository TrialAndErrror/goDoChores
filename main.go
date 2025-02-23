package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"goDoChores/auth"
	"goDoChores/models"
	"goDoChores/views"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/jwtauth/v5"
)

var tokenAuth *jwtauth.JWTAuth

func init() {
	tokenAuth = jwtauth.New("HS256", []byte("secret"), nil)

	_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{"user_id": 123})
	fmt.Printf("DEBUG: a sample jwt is %s\n\n", tokenString)
	err := auth.MakeSampleUser()
	if err != nil {
		return
	}
}

func main() {
	db, dbConnectErr := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if dbConnectErr != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	migrateErr := db.AutoMigrate(&models.Chore{}, &models.ChoreReminder{}, &models.User{})
	if migrateErr != nil {
		panic("failed to migrate database")
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	// Login routes
	r.Group(func(r chi.Router) {
		r.Get("/login", auth.LoginGet)
		r.Post("/login", auth.LoginPost)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(auth.Authenticator(tokenAuth))

		r.Get("/", views.Home)
		r.Post("/", views.HomePost)

		r.Get("/chores/", views.ChoresList)
		r.Get("/chores/new", views.ChoresCreateGet)
		r.Post("/chores/new", views.ChoresCreatePost)
		r.Get("/chores/{choreID}", views.ChoresDetail)

		r.Get("/reminders/", views.RemindersList)
		r.Get("/reminders/new", views.RemindersCreateGet)
		r.Post("/reminders/new", views.RemindersCreatePost)
		r.Get("/reminders/{reminderID}", views.RemindersDetail)
	})

	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	// Get the port from the environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Fatal(http.ListenAndServe(":"+port, r))
}
