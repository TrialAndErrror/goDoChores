package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"goDoChores/auth"
	"goDoChores/models"
	"goDoChores/views/chores"
	"goDoChores/views/home"
	"goDoChores/views/reminders"
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

		r.Mount("/", home.HomeRouter())
		r.Mount("/chores", chores.ChoresRouter())
		r.Mount("/reminders", reminders.RemindersRouter())

		r.Post("/logout", auth.LogoutPost)
	})

	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file; defaulting to environment variables.")
	}

	// Get the port from the environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	fmt.Println("Serving on port " + port)
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
