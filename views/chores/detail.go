package chores

import (
	"context"
	"goDoChores/auth"
	"goDoChores/models"
	"goDoChores/routes"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func DetailGet(w http.ResponseWriter, r *http.Request) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	choreID := chi.URLParam(r, "choreID")
	var chore models.Chore
	queryResult := db.Preload("Reminders").Where(models.Chore{UserID: userID}).First(&chore, choreID)
	if queryResult.Error != nil {
		http.Error(w, queryResult.Error.Error(), http.StatusInternalServerError)
	}
	component := choresDetailPage(chore)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}
}

func DetailPost(w http.ResponseWriter, r *http.Request) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	choreID := chi.URLParam(r, "choreID")
	var chore models.Chore
	queryResult := db.Preload("Reminders").Where(models.Chore{UserID: userID}).First(&chore, choreID)
	if queryResult.Error != nil {
		http.Error(w, queryResult.Error.Error(), http.StatusInternalServerError)
	}

	parseErr := r.ParseForm()
	if parseErr != nil {
		http.Error(w, parseErr.Error(), http.StatusInternalServerError)
	}

	if r.PostFormValue("action") == "delete" {
		db.Delete(&chore)
		http.Redirect(w, r, routes.URLFor("choresList"), http.StatusFound)
	}

	component := choresDetailPage(chore)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}

}
