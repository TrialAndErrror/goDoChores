package home

import (
	"context"
	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/routes"
	"goDoChores/views/reminders"
	"net/http"
	"sort"
	"time"
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
	db := database.DB

	var tasks []models.Task
	if result := db.Where("user_id = ?", userID).Order("date ASC").Limit(6).Find(&tasks); result.Error != nil {
		return nil, result.Error
	}

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
		http.Error(w, parseErr.Error(), http.StatusBadRequest)
		return
	}
	reminderID := r.PostForm.Get("reminderID")
	if reminderID == "" {
		http.Error(w, "ReminderID required", http.StatusBadRequest)
		return
	}

	db := database.DB

	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var reminder models.ChoreReminder
	reminderQueryResult := db.Where("user_id = ?", userID).First(&reminder, reminderID)
	if reminderQueryResult.Error != nil {
		http.Error(w, "Reminder not found", http.StatusNotFound)
		return
	}

	switch r.PostForm.Get("action") {
	case "delete":
		if result := db.Delete(&reminder); result.Error != nil {
			http.Error(w, result.Error.Error(), http.StatusInternalServerError)
			return
		}
	case "done":
		if reminder.Interval == "Once" {
			if result := db.Delete(&reminder); result.Error != nil {
				http.Error(w, result.Error.Error(), http.StatusInternalServerError)
				return
			}
			break
		}
		newDate, err := models.GetNextReminderDate(reminder)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		reminder.Date = newDate
		if result := db.Save(&reminder); result.Error != nil {
			http.Error(w, result.Error.Error(), http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, routes.URLFor("home"), http.StatusFound)
}
