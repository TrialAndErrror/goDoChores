package home

import (
	"context"
	"goDoChores/auth"
	"goDoChores/models"
	"goDoChores/routes"
	"goDoChores/views/reminders"
	"log"
	"net/http"
	"sort"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// CombinedItem represents either a chore reminder or a task for display
type CombinedItem struct {
	ID          interface{} // uint for tasks, int for chore reminders
	Name        string
	Date        time.Time
	Type        string // "chore" or "task"
	Description string
	Interval    string // only for chores
}

func GetTasksForHome(userID uint) ([]models.Task, error) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	var tasks []models.Task
	db.Where(models.Task{UserID: userID}).Order("date ASC").Limit(6).Find(&tasks)

	return tasks, nil
}

func GetCombinedItems(userID uint) ([]CombinedItem, error) {
	// Get tasks
	tasks, err := GetTasksForHome(userID)
	if err != nil {
		return nil, err
	}

	// Get chore reminders
	reminders, err := reminders.GetChoreReminderList(userID)
	if err != nil {
		return nil, err
	}

	// Combine into a single slice
	var items []CombinedItem

	// Add tasks
	for _, task := range tasks {
		items = append(items, CombinedItem{
			ID:          task.ID,
			Name:        task.Name,
			Date:        task.Date,
			Type:        "task",
			Description: task.Description,
		})
	}

	// Add chore reminders
	for _, reminder := range reminders {
		items = append(items, CombinedItem{
			ID:       reminder.ReminderID,
			Name:     reminder.Name,
			Date:     reminder.Date,
			Type:     "chore",
			Interval: reminder.Interval,
		})
	}

	// Sort by date (earliest first)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Date.Before(items[j].Date)
	})

	return items, nil
}

func Home(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	combinedItems, err := GetCombinedItems(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	component := home(combinedItems)
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
		if reminder.Interval == "Once" {
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
