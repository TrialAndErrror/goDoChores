package auth

import (
	"errors"
	"github.com/go-chi/jwtauth/v5"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/routes"
	"gorm.io/gorm"
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

// sessionVersionClaim reads the "sv" claim from the request's token.
func sessionVersionClaim(r *http.Request) (uint, error) {
	_, claims, err := jwtauth.FromContext(r.Context())
	if err != nil {
		return 0, err
	}
	sv, ok := claims["sv"].(float64)
	if !ok {
		return 0, errors.New("token has no session version")
	}
	return uint(sv), nil
}

// ValidSession rejects requests whose token is cryptographically valid but no
// longer current: the user was deleted, logged out, or changed their password
// after the token was issued. Tokens from before session versions existed have
// no "sv" claim and are rejected too. It must run after Authenticator and needs
// database.DB to be open.
func ValidSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reject := func(why string, args ...any) {
			log.Printf("ValidSession: "+why, args...)
			http.Redirect(w, r, routes.URLFor("login"), http.StatusFound)
		}

		userID, err := GetCurrentUserID(r)
		if err != nil {
			reject("%v", err)
			return
		}
		tokenVersion, err := sessionVersionClaim(r)
		if err != nil {
			reject("user %d: %v", userID, err)
			return
		}

		var user models.User
		err = database.DB.Select("id", "session_version").First(&user, userID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			reject("user %d no longer exists", userID)
			return
		}
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if user.SessionVersion != tokenVersion {
			reject("user %d: token has been revoked", userID)
			return
		}
		next.ServeHTTP(w, r)
	})
}
