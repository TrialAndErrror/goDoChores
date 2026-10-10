package chores

import (
	"context"
	"goDoChores/auth"
	"goDoChores/models"
	"net/http"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func GetAllChores(userID uint) ([]models.Chore, error) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	var chores []models.Chore
	result := db.Preload("Reminders").Where("user_id = ?", userID).Find(&chores)
	if result.Error != nil {
		return nil, result.Error
	}

	return chores, nil
}

func List(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	chores, err := GetAllChores(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	component := choresList(chores)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}

}
