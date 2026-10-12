package reminders

import (
	"context"
	"net/http"
	"strconv"

	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/routes"

	"github.com/go-chi/chi/v5"
)

func EditGet(w http.ResponseWriter, r *http.Request) {
	reminderID := chi.URLParam(r, "reminderID")
	id, err := strconv.ParseUint(reminderID, 10, 32)
	if err != nil {
		http.Error(w, "Invalid reminder ID", http.StatusBadRequest)
		return
	}

	db := database.DB

	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var reminder models.ChoreReminder
	result := db.Where("user_id = ?", userID).First(&reminder, id)
	if result.Error != nil {
		http.Error(w, "Reminder not found", http.StatusNotFound)
		return
	}

	// Get all chores for the user to populate the dropdown
	var chores []models.Chore
	db.Where(models.Chore{UserID: userID}).Find(&chores)

	sortChoresByName(chores)

	if renderErr := remindersEditPage(reminder, chores, models.GetIntervalNames()).Render(context.Background(), w); renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}
}

func EditPost(w http.ResponseWriter, r *http.Request) {
	reminderID := chi.URLParam(r, "reminderID")
	id, err := strconv.ParseUint(reminderID, 10, 32)
	if err != nil {
		http.Error(w, "Invalid reminder ID", http.StatusBadRequest)
		return
	}

	db := database.DB

	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var reminder models.ChoreReminder
	result := db.Where("user_id = ?", userID).First(&reminder, id)
	if result.Error != nil {
		http.Error(w, "Reminder not found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Validate the submitted fields the same way create does.
	updated, err := models.ChoreReminderFromForm(r.PostForm, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !userOwnsChore(db, userID, updated.ChoreID) {
		http.Error(w, "Chore not found", http.StatusBadRequest)
		return
	}
	reminder.ChoreID = updated.ChoreID
	reminder.Date = updated.Date
	reminder.Interval = updated.Interval
	reminder.IntervalValue = updated.IntervalValue

	// Save the updated reminder
	if err := db.Save(&reminder).Error; err != nil {
		http.Error(w, "Failed to update reminder", http.StatusInternalServerError)
		return
	}

	// Redirect to the reminder detail page
	http.Redirect(w, r, routes.URLFor("remindersDetail", reminder.ID), http.StatusSeeOther)
}
