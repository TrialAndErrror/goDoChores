package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
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
	// Initialise JWT token auth
	tokenAuth = jwtauth.New("HS256", []byte("secret"), nil)
}

func runServer() {
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
		r.Post("/logout", auth.LogoutPost)
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

func main() {
	// Define the root command (runs the server by default)
	var rootCmd = &cobra.Command{
		Use:   "myapp",
		Short: "My Go App CLI and Server",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Starting the server...")
			runServer()
		},
	}

	// Add subcommands
	rootCmd.AddCommand(auth.CreateUserCmd)

	// Execute the CLI
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}
