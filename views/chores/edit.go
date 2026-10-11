package chores

import (
	"context"
	"net/http"
	"strconv"

	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/routes"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
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
	result := db.Preload("Tags").Where("user_id = ?", userID).First(&chore, id)
	if result.Error != nil {
		http.Error(w, "Chore not found", http.StatusNotFound)
		return
	}

	selected := make([]uint, 0, len(chore.Tags))
	for _, t := range chore.Tags {
		selected = append(selected, t.TagID)
	}
	picker, err := newTagPicker(userID, selected)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if renderErr := choresEditForm(chore, picker).Render(context.Background(), w); renderErr != nil {
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

	// Validate the submitted fields the same way create does.
	updated, err := models.ChoreFromForm(r.PostForm, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	chore.Name = updated.Name
	chore.Description = updated.Description
	chore.Time = updated.Time
	tags, err := tagIDsFromForm(userID, r.PostForm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Save the updated chore and its tags together.
	saveErr := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Tags").Save(&chore).Error; err != nil {
			return err
		}
		return saveTags(tx, chore.ID, tags)
	})
	if saveErr != nil {
		http.Error(w, "Failed to update chore", http.StatusInternalServerError)
		return
	}

	// Redirect to the chore detail page
	http.Redirect(w, r, routes.URLFor("choresDetail", chore.ID), http.StatusSeeOther)
}
