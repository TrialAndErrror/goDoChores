package chores

import (
	"context"
	"goDoChores/auth"
	"goDoChores/models"
	"goDoChores/routes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
)

func CreateGet(w http.ResponseWriter, r *http.Request) {
	component := choresCreatePage()
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}

}

func CreatePost(w http.ResponseWriter, r *http.Request) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	parseError := r.ParseForm()
	if parseError != nil {
		http.Error(w, parseError.Error(), http.StatusBadRequest)
		return
	}

	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	chore, choreCreateErr := models.ChoreFromForm(r.PostForm, userID)
	if choreCreateErr != nil {
		http.Error(w, choreCreateErr.Error(), http.StatusBadRequest)
		return
	}

	if result := db.Create(&chore); result.Error != nil {
		http.Error(w, result.Error.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, routes.URLFor("choresDetail", chore.ID), http.StatusFound)
}
