package views

import (
	"context"
	"goDoChores/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
)

func Home(w http.ResponseWriter, r *http.Request) {
	component := home("Jerry")
	err := component.Render(context.Background(), w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func ChoresList(w http.ResponseWriter, r *http.Request) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var chores []models.Chore
	db.Find(&chores)

	component := choresList(chores)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}

}
