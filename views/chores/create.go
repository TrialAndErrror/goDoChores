package chores

import (
	"context"
	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/routes"
	"net/http"

	"gorm.io/gorm"
)

func CreateGet(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	picker, err := newTagPicker(userID, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	component := choresCreatePage(picker)
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
	chore, choreCreateErr := models.ChoreFromForm(r.PostForm, userID)
	if choreCreateErr != nil {
		http.Error(w, choreCreateErr.Error(), http.StatusBadRequest)
		return
	}

	tags, tagsErr := tagIDsFromForm(userID, r.PostForm)
	if tagsErr != nil {
		http.Error(w, tagsErr.Error(), http.StatusBadRequest)
		return
	}

	createErr := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&chore).Error; err != nil {
			return err
		}
		return saveTags(tx, chore.ID, tags)
	})
	if createErr != nil {
		http.Error(w, createErr.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, routes.URLFor("choresDetail", chore.ID), http.StatusFound)
}
