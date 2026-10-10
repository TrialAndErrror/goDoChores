package chores

import (
	"context"
	"net/http"
	"strconv"

	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"

	"github.com/go-chi/chi/v5"
)

func EditGet(w http.ResponseWriter, r *http.Request) {
	choreID := chi.URLParam(r, "choreID")
	id, err := strconv.ParseUint(choreID, 10, 32)
	if err != nil {
		http.Error(w, "Invalid chore ID", http.StatusBadRequest)
		return
	}

	db := database.DB

	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var chore models.Chore
	result := db.Where("user_id = ?", userID).First(&chore, id)
	if result.Error != nil {
		http.Error(w, "Chore not found", http.StatusNotFound)
		return
	}

	if renderErr := choresEditForm(chore).Render(context.Background(), w); renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}
}

func EditPost(w http.ResponseWriter, r *http.Request) {
	choreID := chi.URLParam(r, "choreID")
	id, err := strconv.ParseUint(choreID, 10, 32)
	if err != nil {
		http.Error(w, "Invalid chore ID", http.StatusBadRequest)
		return
	}

	db := database.DB

	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var chore models.Chore
	result := db.Where("user_id = ?", userID).First(&chore, id)
	if result.Error != nil {
		http.Error(w, "Chore not found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Update chore fields
	chore.Name = r.FormValue("name")
	chore.Description = r.FormValue("description")

	timeStr := r.FormValue("time")
	if timeInt, err := strconv.Atoi(timeStr); err == nil {
		chore.Time = timeInt
	}

	// Save the updated chore
	if err := db.Save(&chore).Error; err != nil {
		http.Error(w, "Failed to update chore", http.StatusInternalServerError)
		return
	}

	// Redirect to the chore detail page
	http.Redirect(w, r, "/chores/"+choreID+"/", http.StatusSeeOther)
}
