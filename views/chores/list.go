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
	db.Preload("Reminders").Where(models.Chore{UserID: userID}).Find(&chores)

	return chores, nil
}

func List(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	chores, err := GetAllChores(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	component := choresList(chores)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}

}
