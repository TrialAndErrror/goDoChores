package reminders

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"goDoChores/auth"
	"goDoChores/models"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func EditGet(w http.ResponseWriter, r *http.Request) {
	reminderID := chi.URLParam(r, "reminderID")
	id, err := strconv.ParseUint(reminderID, 10, 32)
	if err != nil {
		http.Error(w, "Invalid reminder ID", http.StatusBadRequest)
		return
	}

	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var reminder models.ChoreReminder
	result := db.First(&reminder, id)
	if result.Error != nil {
		http.Error(w, "Reminder not found", http.StatusNotFound)
		return
	}

	// Get all chores for the user to populate the dropdown
	var chores []models.Chore
	db.Where(models.Chore{UserID: userID}).Find(&chores)

	remindersEditPage(reminder, chores, models.GetIntervalNames()).Render(context.Background(), w)
}

func EditPost(w http.ResponseWriter, r *http.Request) {
	reminderID := chi.URLParam(r, "reminderID")
	id, err := strconv.ParseUint(reminderID, 10, 32)
	if err != nil {
		http.Error(w, "Invalid reminder ID", http.StatusBadRequest)
		return
	}

	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	var reminder models.ChoreReminder
	result := db.First(&reminder, id)
	if result.Error != nil {
		http.Error(w, "Reminder not found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Update reminder fields
	choreIDStr := r.FormValue("choreID")
	if choreID, err := strconv.ParseUint(choreIDStr, 10, 32); err == nil {
		reminder.ChoreID = choreID
	}

	dateStr := r.FormValue("date")
	dateFormatString := "2006-01-02"
	if date, err := time.Parse(dateFormatString, dateStr); err == nil {
		reminder.Date = date
	}

	reminder.Interval = r.FormValue("interval")

	// Save the updated reminder
	if err := db.Save(&reminder).Error; err != nil {
		http.Error(w, "Failed to update reminder", http.StatusInternalServerError)
		return
	}

	// Redirect to the reminder detail page
	http.Redirect(w, r, "/reminders/"+reminderID+"/", http.StatusSeeOther)
}
