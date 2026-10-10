package auth

import (
	"github.com/go-chi/jwtauth/v5"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/routes"
	"log"
	"net/http"
)

func Authenticator(ja *jwtauth.JWTAuth) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			token, _, err := jwtauth.FromContext(r.Context())

			if err != nil {
				log.Printf("Authenticator Err: Error parsing token: %v", err)
				http.Redirect(w, r, routes.URLFor("login"), http.StatusFound)
				return
			}

			if token == nil {
				log.Printf("Authenticator Err: No Token")
				http.Redirect(w, r, routes.URLFor("login"), http.StatusFound)
				return
			}

			// Token is authenticated, pass it through
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(hfn)
	}
}

// UserExists rejects requests whose token is valid but belongs to a user that
// no longer exists (e.g. deleted after the token was issued). It must run after
// Authenticator, and needs database.DB to be open.
func UserExists(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := GetCurrentUserID(r)
		if err != nil {
			log.Printf("UserExists Err: %v", err)
			http.Redirect(w, r, routes.URLFor("login"), http.StatusFound)
			return
		}

		var count int64
		if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Count(&count).Error; err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if count == 0 {
			log.Printf("UserExists Err: user %d no longer exists", userID)
			http.Redirect(w, r, routes.URLFor("login"), http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}
