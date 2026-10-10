package main

import (
	"fmt"
	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/views/chores"
	"goDoChores/views/home"
	"goDoChores/views/reminders"
	"goDoChores/views/tasks"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/jwtauth/v5"
)

func runServer() {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET must be set (see .env.example)")
	}
	tokenAuth := auth.Init(jwtSecret)

	if err := database.Open(); err != nil {
		log.Fatal(err)
	}

	// Migrate the schema
	migrateErr := database.DB.AutoMigrate(&models.Chore{}, &models.ChoreReminder{}, &models.Task{}, &models.User{})
	if migrateErr != nil {
		log.Fatalf("failed to migrate database: %v", migrateErr)
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
		r.Mount("/tasks", tasks.TasksRouter())

		r.Post("/logout", auth.LogoutPost)
	})

	// Get the port from the environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	fmt.Println("Serving on port " + port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func main() {
	// Load .env (if any) up front so every command, including create-user,
	// sees the same configuration. Real environment variables still win.
	_ = godotenv.Load()

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
