package chores

import (
	"context"
	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/routes"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
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
	queryResult := db.Preload("Reminders").Preload("Tags.Tag").Where("user_id = ?", userID).First(&chore, choreID)
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
	queryResult := db.Preload("Reminders").Preload("Tags.Tag").Where("user_id = ?", userID).First(&chore, choreID)
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
		// Delete the chore together with its reminders so none are left
		// pointing at a chore that no longer exists.
		deleteErr := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("chore_id = ? AND user_id = ?", chore.ID, userID).Delete(&models.ChoreReminder{}).Error; err != nil {
				return err
			}
			if err := tx.Where("chore_id = ?", chore.ID).Delete(&models.ChoreTag{}).Error; err != nil {
				return err
			}
			return tx.Delete(&chore).Error
		})
		if deleteErr != nil {
			http.Error(w, deleteErr.Error(), http.StatusInternalServerError)
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
