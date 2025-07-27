package tasks

import (
	"context"
	"goDoChores/auth"
	"goDoChores/models"
	"net/http"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func GetAllTasks(userID uint) ([]models.Task, error) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	var tasks []models.Task
	db.Where(models.Task{UserID: userID}).Order("date ASC").Find(&tasks)

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
