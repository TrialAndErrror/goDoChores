package reminders

import (
	"cmp"
	"context"
	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/routes"
	"goDoChores/views/chores"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// userOwnsChore reports whether the chore exists and belongs to the user.
func userOwnsChore(db *gorm.DB, userID uint, choreID uint64) bool {
	var count int64
	db.Model(&models.Chore{}).Where("id = ? AND user_id = ?", choreID, userID).Count(&count)
	return count > 0
}

// sortChoresByName orders chores for a dropdown: alphabetical, ignoring case.
func sortChoresByName(chores []models.Chore) {
	slices.SortStableFunc(chores, func(a, b models.Chore) int {
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
}

func CreateGet(w http.ResponseWriter, r *http.Request) {

	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	chores, err := chores.GetAllChores(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sortChoresByName(chores)
	// The chore page links here with ?choreID= to preselect its chore. An id
	// that is missing, malformed or not one of the user's chores selects nothing.
	selected, _ := strconv.ParseUint(r.URL.Query().Get("choreID"), 10, 32)
	component := remindersCreatePage(chores, models.GetIntervalNames(), uint(selected))
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}

}
func CreatePost(w http.ResponseWriter, r *http.Request) {
	db := database.DB
	parseError := r.ParseForm()
	if parseError != nil {
		http.Error(w, parseError.Error(), http.StatusBadRequest)
		return
	}

	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	choreReminder, choreCreateErr := models.ChoreReminderFromForm(r.PostForm, userID)
	if choreCreateErr != nil {
		http.Error(w, choreCreateErr.Error(), http.StatusBadRequest)
		return
	}

	if !userOwnsChore(db, userID, choreReminder.ChoreID) {
		http.Error(w, "Chore not found", http.StatusBadRequest)
		return
	}

	if result := db.Create(&choreReminder); result.Error != nil {
		http.Error(w, result.Error.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, routes.URLFor("remindersDetail", choreReminder.ID), http.StatusFound)
}
