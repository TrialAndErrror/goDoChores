package tasks

import (
	"context"
	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"
	"net/http"
)

func GetAllTasks(userID uint) ([]models.Task, error) {
	db := database.DB

	var tasks []models.Task
	if result := db.Where("user_id = ?", userID).Order("date ASC").Find(&tasks); result.Error != nil {
		return nil, result.Error
	}

	return tasks, nil
}

func List(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tasks, err := GetAllTasks(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	component := tasksList(tasks)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}
}
