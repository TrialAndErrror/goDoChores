package reminders

import (
	"context"
	"github.com/go-chi/chi/v5"
	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/routes"
	"net/http"
)

func DetailGet(w http.ResponseWriter, r *http.Request) {
	db := database.DB

	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	reminderID := chi.URLParam(r, "reminderID")
	var reminder models.ChoreReminder
	reminderQueryResult := db.Where("user_id = ?", userID).First(&reminder, reminderID)
	if reminderQueryResult.Error != nil {
		http.Error(w, "Reminder not found", http.StatusNotFound)
		return
	}

	var chore models.Chore
	choreQueryResult := db.First(&chore, reminder.ChoreID)
	if choreQueryResult.Error != nil {
		http.Error(w, "Chore not found", http.StatusNotFound)
		return
	}

	component := remindersDetailPage(chore, reminder)
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
	reminderID := chi.URLParam(r, "reminderID")
	var reminder models.ChoreReminder
	reminderQueryResult := db.Where("user_id = ?", userID).First(&reminder, reminderID)
	if reminderQueryResult.Error != nil {
		http.Error(w, "Reminder not found", http.StatusNotFound)
		return
	}

	var chore models.Chore
	choreQueryResult := db.First(&chore, reminder.ChoreID)
	if choreQueryResult.Error != nil {
		http.Error(w, "Chore not found", http.StatusNotFound)
		return
	}

	parseErr := r.ParseForm()
	if parseErr != nil {
		http.Error(w, parseErr.Error(), http.StatusBadRequest)
		return
	}

	if r.PostFormValue("action") == "delete" {
		if result := db.Delete(&reminder); result.Error != nil {
			http.Error(w, result.Error.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, routes.URLFor("remindersList"), http.StatusFound)
		return
	}

	component := remindersDetailPage(chore, reminder)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}
}
