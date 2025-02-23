package auth

import (
	"github.com/go-chi/jwtauth/v5"
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
			}

			if token == nil {
				log.Printf("Authenticator Err: No Token")
				http.Redirect(w, r, routes.URLFor("login"), http.StatusFound)
			}

			// Token is authenticated, pass it through
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(hfn)
	}
}
