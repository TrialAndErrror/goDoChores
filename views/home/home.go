package home

import (
	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/routes"
	"goDoChores/views/reminders"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// CombinedItem represents either a chore reminder or a task for display
type CombinedItem struct {
	ID          interface{} // task ID for tasks, reminder ID for chore reminders
	Name        string
	Date        time.Time
	Type        string // "chore" or "task"
	Description string
	Interval    string // only for chores
}

const (
	// initialBatch is how many items the home page renders up front.
	initialBatch = 12
	// moreBatch is how many items each infinite-scroll request adds.
	moreBatch = 6
)

func GetTasksForHome(userID uint) ([]models.Task, error) {
	db := database.DB

	var tasks []models.Task
	if result := db.Where("user_id = ?", userID).Order("date ASC").Find(&tasks); result.Error != nil {
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

	sortItems(items)

	return items, nil
}

// sortItems orders items by date, earliest first. Items on the same date are
// ordered by type and ID so the order is identical on every request; otherwise
// a batch boundary inside a run of equal dates could repeat or skip items.
func sortItems(items []CombinedItem) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if !a.Date.Equal(b.Date) {
			return a.Date.Before(b.Date)
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return idNumber(a.ID) < idNumber(b.ID)
	})
}

func idNumber(id interface{}) uint64 {
	switch v := id.(type) {
	case uint:
		return uint64(v)
	case int:
		return uint64(v)
	}
	return 0
}

// pageItems returns the slice of items to render for the given offset, the
// offset of the following batch, and whether any items remain after this one.
// Offset 0 is the first page (initialBatch items); later offsets get moreBatch.
func pageItems(items []CombinedItem, offset int) (page []CombinedItem, nextOffset int, hasMore bool) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(items) {
		offset = len(items)
	}
	size := moreBatch
	if offset == 0 {
		size = initialBatch
	}
	end := min(offset+size, len(items))
	return items[offset:end], end, end < len(items)
}

func Home(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			http.Error(w, "Invalid offset", http.StatusBadRequest)
			return
		}
	}

	combinedItems, err := GetCombinedItems(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	page, nextOffset, hasMore := pageItems(combinedItems, offset)

	// Later batches (infinite scroll) are just the cards, not a full page.
	component := home(page, nextOffset, hasMore)
	if offset > 0 {
		component = itemsBatch(page, nextOffset, hasMore)
	}
	renderErr := component.Render(r.Context(), w)
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
