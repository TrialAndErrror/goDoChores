package reminders

import (
	"context"
	"goDoChores/auth"
	"goDoChores/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"time"
)

type ChoreReminderListEntry struct {
	ReminderID int
	Interval   string
	Date       time.Time
	ChoreID    int
	Name       string
}

func GetChoreReminderList(userID uint) ([]ChoreReminderListEntry, error) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	var reminders []ChoreReminderListEntry
	db.Model(&models.ChoreReminder{}).Select("chore_reminders.id as reminder_id, chore_reminders.interval,chore_reminders.date, chores.id as chore_id, chores.name").Joins("left join chores on chore_reminders.chore_id = chores.id").Where("chore_reminders.user_id = ?", userID).Scan(&reminders)
	return reminders, nil
}

func List(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	reminders, err := GetChoreReminderList(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	component := remindersList(reminders)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}

}
