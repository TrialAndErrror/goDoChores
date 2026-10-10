package main

import (
	"fmt"
	"goDoChores/auth"
	"goDoChores/cli"
	"goDoChores/database"
	"goDoChores/views/chores"
	"goDoChores/views/home"
	"goDoChores/views/reminders"
	"goDoChores/views/tasks"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/jwtauth/v5"
)

// newRouter builds the full HTTP handler tree: public login routes plus the
// JWT-protected app routes.
func newRouter(tokenAuth *jwtauth.JWTAuth, csrf *http.CrossOriginProtection) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	// Reject cross-site form posts (CSRF) before any handler sees them.
	r.Use(csrf.Handler)

	// Login routes
	r.Group(func(r chi.Router) {
		r.Get("/login", auth.LoginGet)
		r.Post("/login", auth.LoginPost)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(auth.Authenticator(tokenAuth))
		r.Use(auth.ValidSession)

		r.Mount("/", home.HomeRouter())
		r.Mount("/chores", chores.ChoresRouter())
		r.Mount("/reminders", reminders.RemindersRouter())
		r.Mount("/tasks", tasks.TasksRouter())

		r.Post("/logout", auth.LogoutPost)
	})

	return r
}

// newCrossOriginProtection builds the CSRF guard. Browsers are checked via
// Sec-Fetch-Site, or by comparing the Origin header with the Host header. If a
// reverse proxy rewrites Host, list the public origin(s), comma-separated,
// in TRUSTED_ORIGINS (e.g. "https://chores.example.com").
func newCrossOriginProtection(trustedOrigins string) (*http.CrossOriginProtection, error) {
	csrf := http.NewCrossOriginProtection()
	for _, origin := range strings.Split(trustedOrigins, ",") {
		if origin = strings.TrimSpace(origin); origin == "" {
			continue
		}
		if err := csrf.AddTrustedOrigin(origin); err != nil {
			return nil, fmt.Errorf("TRUSTED_ORIGINS: %w", err)
		}
	}
	return csrf, nil
}

func runServer() {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET must be set (see .env.example)")
	}
	tokenAuth := auth.Init(jwtSecret)

	if err := database.Open(); err != nil {
		log.Fatal(err)
	}

	if err := database.Migrate(); err != nil {
		log.Fatal(err)
	}

	csrf, err := newCrossOriginProtection(os.Getenv("TRUSTED_ORIGINS"))
	if err != nil {
		log.Fatal(err)
	}
	r := newRouter(tokenAuth, csrf)

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
		Use:   "goDoChores",
		Short: "goDoChores web server and CLI",
		// Subcommands report their own errors; main prints them once.
		SilenceErrors: true,
		SilenceUsage:  true,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Starting the server...")
			runServer()
		},
	}

	// Add subcommands
	rootCmd.AddCommand(cli.Commands()...)

	// Execute the CLI
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

}
