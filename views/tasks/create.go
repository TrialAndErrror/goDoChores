package tasks

import (
	"context"
	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/routes"
	"net/http"
)

func CreateGet(w http.ResponseWriter, r *http.Request) {
	component := tasksCreateForm()
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}
}

func CreatePost(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	task, err := models.TaskFromForm(r.Form, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db := database.DB

	result := db.Create(&task)
	if result.Error != nil {
		http.Error(w, result.Error.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, routes.URLFor("tasksDetail", task.ID), http.StatusSeeOther)
}
