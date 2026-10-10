package chores

import (
	"context"
	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/routes"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func DetailGet(w http.ResponseWriter, r *http.Request) {
	db := database.DB
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	choreID := chi.URLParam(r, "choreID")
	var chore models.Chore
	queryResult := db.Preload("Reminders").Where("user_id = ?", userID).First(&chore, choreID)
	if queryResult.Error != nil {
		http.Error(w, "Chore not found", http.StatusNotFound)
		return
	}
	component := choresDetailPage(chore)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}
}

func DetailPost(w http.ResponseWriter, r *http.Request) {
	db := database.DB
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	choreID := chi.URLParam(r, "choreID")
	var chore models.Chore
	queryResult := db.Preload("Reminders").Where("user_id = ?", userID).First(&chore, choreID)
	if queryResult.Error != nil {
		http.Error(w, "Chore not found", http.StatusNotFound)
		return
	}

	parseErr := r.ParseForm()
	if parseErr != nil {
		http.Error(w, parseErr.Error(), http.StatusBadRequest)
		return
	}

	if r.PostFormValue("action") == "delete" {
		if result := db.Delete(&chore); result.Error != nil {
			http.Error(w, result.Error.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, routes.URLFor("choresList"), http.StatusFound)
		return
	}

	component := choresDetailPage(chore)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}

}
