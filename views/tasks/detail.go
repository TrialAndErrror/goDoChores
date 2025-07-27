package tasks

import (
	"context"
	"goDoChores/auth"
	"goDoChores/models"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func GetTaskByID(taskID uint, userID uint) (*models.Task, error) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	var task models.Task
	result := db.Where("id = ? AND user_id = ?", taskID, userID).First(&task)
	if result.Error != nil {
		return nil, result.Error
	}

	return &task, nil
}

func DetailGet(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	taskIDStr := chi.URLParam(r, "taskID")
	if taskIDStr == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	task, err := GetTaskByID(uint(taskID), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	component := tasksDetail(*task)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}
}

func DetailPost(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	taskIDStr := chi.URLParam(r, "taskID")
	if taskIDStr == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	// Get the task
	task, err := GetTaskByID(uint(taskID), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Handle different actions
	action := r.PostForm.Get("action")
	switch action {
	case "delete":
		// Delete the task
		db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := db.Delete(task).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Redirect to tasks list after deletion
	http.Redirect(w, r, "/tasks/", http.StatusFound)
}
