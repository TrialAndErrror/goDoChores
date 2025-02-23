package reminders

import (
	"context"
	"github.com/go-chi/chi/v5"
	"goDoChores/auth"
	"goDoChores/models"
	"goDoChores/routes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
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
	reminderID := chi.URLParam(r, "reminderID")
	var reminder models.ChoreReminder
	reminderQueryResult := db.Where(models.ChoreReminder{UserID: userID}).First(&reminder, reminderID)
	if reminderQueryResult.Error != nil {
		http.Error(w, reminderQueryResult.Error.Error(), http.StatusInternalServerError)
	}

	var chore models.Chore
	choreQueryResult := db.First(&chore, reminder.ChoreID)
	if choreQueryResult.Error != nil {
		http.Error(w, choreQueryResult.Error.Error(), http.StatusInternalServerError)
	}

	component := remindersDetailPage(chore, reminder)
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
	reminderID := chi.URLParam(r, "reminderID")
	var reminder models.ChoreReminder
	reminderQueryResult := db.Where(models.ChoreReminder{UserID: userID}).First(&reminder, reminderID)
	if reminderQueryResult.Error != nil {
		http.Error(w, reminderQueryResult.Error.Error(), http.StatusInternalServerError)
	}

	var chore models.Chore
	choreQueryResult := db.First(&chore, reminder.ChoreID)
	if choreQueryResult.Error != nil {
		http.Error(w, choreQueryResult.Error.Error(), http.StatusInternalServerError)
	}

	parseErr := r.ParseForm()
	if parseErr != nil {
		http.Error(w, parseErr.Error(), http.StatusInternalServerError)
	}

	if r.PostFormValue("action") == "delete" {
		db.Delete(&reminder)
		http.Redirect(w, r, routes.URLFor("remindersList"), http.StatusFound)
	}

	component := remindersDetailPage(chore, reminder)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}
}
