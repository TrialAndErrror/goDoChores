package auth

import (
	"context"
	"errors"
	"github.com/go-chi/jwtauth/v5"
	"goDoChores/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"net/http"
	"time"
)

var tokenAuth *jwtauth.JWTAuth

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

	tokenAuth = jwtauth.New("HS256", []byte("secret"), nil)

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
		http.Error(w, parseErr.Error(), http.StatusInternalServerError)
	}
	log.Printf("%v", r.PostForm)

	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")

	token, err := LoginUser(username, password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Create a cookie to store the token.
	cookie := http.Cookie{
		Name:     "jwt",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, &cookie)

	// Redirect the user to the dashboard or another protected page.
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
