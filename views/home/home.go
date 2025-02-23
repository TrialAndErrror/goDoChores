package home

import (
	"context"
	"goDoChores/auth"
	"goDoChores/models"
	"goDoChores/routes"
	"goDoChores/views/reminders"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"net/http"
)

func Home(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	remindersList, reminderListErr := reminders.GetChoreReminderList(userID)
	if reminderListErr != nil {
		http.Error(w, reminderListErr.Error(), http.StatusInternalServerError)
	}
	component := home(remindersList)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}
}

func HomePost(w http.ResponseWriter, r *http.Request) {
	parseErr := r.ParseForm()
	if parseErr != nil {
		http.Error(w, parseErr.Error(), http.StatusInternalServerError)
	}
	log.Printf("%v", r.PostForm)
	reminderID := r.PostForm.Get("reminderID")
	if reminderID == "" {
		http.Error(w, "ReminderID required", http.StatusBadRequest)
		return
	}

	db, dbErr := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if dbErr != nil {
		http.Error(w, dbErr.Error(), http.StatusInternalServerError)
		return
	}

	var reminder models.ChoreReminder
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	reminderQueryResult := db.Where(models.ChoreReminder{UserID: userID}).First(&reminder, reminderID)
	if reminderQueryResult.Error != nil {
		http.Error(w, reminderQueryResult.Error.Error(), http.StatusInternalServerError)
		return
	}
	action := r.PostForm.Get("action")
	switch action {
	case "delete":
		db.Delete(&reminder, reminderID)
		break
	case "done":
		if reminder.Interval == "once" {
			db.Delete(&reminder, reminderID)
		} else {
			newDate, err := models.GetNextReminderDate(reminder)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			} else {
				reminder.Date = newDate
				db.Save(&reminder)
			}
		}
		break
	}

	http.Redirect(w, r, routes.URLFor("home"), http.StatusFound)
	return

}
