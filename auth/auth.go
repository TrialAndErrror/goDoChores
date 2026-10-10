package auth

import (
	"context"
	"errors"
	"github.com/go-chi/jwtauth/v5"
	"goDoChores/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"os"
	"time"
)

var tokenAuth *jwtauth.JWTAuth

// Init sets up the shared JWT signer/verifier. It must be called once at
// startup, before any request is served, and returns the instance so the
// router can use the same one for verification.
func Init(secret string) *jwtauth.JWTAuth {
	tokenAuth = jwtauth.New("HS256", []byte(secret), nil)
	return tokenAuth
}

// secureCookies reports whether the jwt cookie should be marked Secure.
// Set COOKIE_SECURE=true when serving over HTTPS.
func secureCookies() bool {
	return os.Getenv("COOKIE_SECURE") == "true"
}

func LoginUser(username string, password string) (string, error) {
	db, dbErr := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if dbErr != nil {
		return "", errors.New("cannot connect to database")
	}
	var user models.User
	tx := db.First(&user, "username = ?", username)
	if tx.Error != nil {
		return "", errors.New("invalid username or password")
	}

	if !user.CheckPassword(password) {
		return "", errors.New("invalid username or password")
	}

	if tokenAuth == nil {
		return "", errors.New("auth not initialised")
	}

	_, tokenString, err := tokenAuth.Encode(map[string]interface{}{"user_id": user.ID})
	if err != nil {
		return "", errors.New("failed to generate token")
	}
	return tokenString, nil
}

func LoginGet(w http.ResponseWriter, r *http.Request) {
	component := login()
	err := component.Render(context.Background(), w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func LoginPost(w http.ResponseWriter, r *http.Request) {

	parseErr := r.ParseForm()
	if parseErr != nil {
		http.Error(w, parseErr.Error(), http.StatusBadRequest)
		return
	}

	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")

	token, err := LoginUser(username, password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Create a cookie to store the token.
	cookie := http.Cookie{
		Name:     "jwt",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Secure:   secureCookies(),
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, &cookie)

	// Redirect the user to the dashboard or another protected page.
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func LogoutPost(w http.ResponseWriter, r *http.Request) {
	// Create an expired cookie with the same name
	cookie := http.Cookie{
		Name:     "jwt", // Cookie name must match the one used in login
		Value:    "",    // Empty value
		Path:     "/",
		Expires:  time.Unix(0, 0), // Set expiry in the past
		HttpOnly: true,
		Secure:   secureCookies(),
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, &cookie)

	// Redirect to the login page or respond with a success message
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
