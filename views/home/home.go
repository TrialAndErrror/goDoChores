package home

import (
	"github.com/a-h/templ"
	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/routes"
	"goDoChores/views/groups"
	"net/http"
	"slices"
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
	Time        string // freeform, chores only
	Owner       string // username of another group member; empty for your own items
}

// groupReminder is a chore reminder together with who owns it.
type groupReminder struct {
	ReminderID  int
	Interval    string
	Date        time.Time
	ChoreID     uint
	Name        string
	Description string
	Time        string
	Owner       string
	UserID      uint
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

// visibleReminders returns the reminders of the user and of the other members
// of their group, each with its owner's username.
func visibleReminders(userID uint) ([]groupReminder, error) {
	db := database.DB
	others, err := groups.OtherMemberIDs(db, userID)
	if err != nil {
		return nil, err
	}
	ids := append([]uint{userID}, others...)

	var rows []groupReminder
	err = db.Model(&models.ChoreReminder{}).
		Select("chore_reminders.id as reminder_id, chore_reminders.interval, chore_reminders.date, chores.id as chore_id, chores.name, chores.description, chores.time, users.username as owner, chore_reminders.user_id").
		Joins("join chores on chore_reminders.chore_id = chores.id AND chores.deleted_at IS NULL").
		Joins("join users on users.id = chore_reminders.user_id AND users.deleted_at IS NULL").
		Where("chore_reminders.user_id IN ?", ids).
		Where("chore_reminders.deleted_at IS NULL").
		Scan(&rows).Error
	return rows, err
}

// choreTags maps each chore ID to the ids of its tags.
func choreTags(choreIDs []uint) (map[uint][]uint, error) {
	out := map[uint][]uint{}
	if len(choreIDs) == 0 {
		return out, nil
	}
	var tags []models.ChoreTag
	if err := database.DB.Where("chore_id IN ?", choreIDs).Find(&tags).Error; err != nil {
		return nil, err
	}
	for _, t := range tags {
		out[t.ChoreID] = append(out[t.ChoreID], t.TagID)
	}
	return out, nil
}

func GetCombinedItems(userID uint) ([]CombinedItem, error) {
	// Get tasks
	tasks, err := GetTasksForHome(userID)
	if err != nil {
		return nil, err
	}

	// Get chore reminders: the user's own plus their group's, minus anything
	// carrying a tag the user has filtered out.
	reminders, err := visibleReminders(userID)
	if err != nil {
		return nil, err
	}
	hidden, err := groups.HiddenTags(database.DB, userID)
	if err != nil {
		return nil, err
	}
	choreIDs := make([]uint, 0, len(reminders))
	for _, rem := range reminders {
		choreIDs = append(choreIDs, rem.ChoreID)
	}
	tagsByChore, err := choreTags(choreIDs)
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
		if slices.ContainsFunc(tagsByChore[reminder.ChoreID], func(t uint) bool { return hidden[t] }) {
			continue
		}
		item := CombinedItem{
			ID:          reminder.ReminderID,
			Name:        reminder.Name,
			Date:        reminder.Date,
			Type:        "chore",
			Interval:    reminder.Interval,
			Description: reminder.Description,
			Time:        reminder.Time,
		}
		if reminder.UserID != userID {
			item.Owner = reminder.Owner
		}
		items = append(items, item)
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
	var component templ.Component
	if offset > 0 {
		component = itemsBatch(page, nextOffset, hasMore)
	} else {
		invites, err := groups.PendingInviteCount(database.DB, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		component = home(page, nextOffset, hasMore, invites)
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

	// Your own reminders can be completed or deleted. Reminders owned by other
	// members of your group can be completed too, but never deleted or edited.
	owners := []uint{userID}
	if others, err := groups.OtherMemberIDs(db, userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		owners = append(owners, others...)
	}
	var reminder models.ChoreReminder
	reminderQueryResult := db.Where("user_id IN ?", owners).First(&reminder, reminderID)
	if reminderQueryResult.Error != nil {
		http.Error(w, "Reminder not found", http.StatusNotFound)
		return
	}
	action := r.PostForm.Get("action")
	if action == "delete" && reminder.UserID != userID {
		http.Error(w, "Only the owner can delete a reminder", http.StatusForbidden)
		return
	}

	switch action {
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
