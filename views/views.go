package views

import (
	"context"
	"github.com/go-chi/chi/v5"
	"goDoChores/models"
	"goDoChores/routes"
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

func ChoresCreateGet(w http.ResponseWriter, r *http.Request) {
	component := choresCreatePage()
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}

}

func ChoresCreatePost(w http.ResponseWriter, r *http.Request) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	parseError := r.ParseForm()
	if parseError != nil {
		http.Error(w, parseError.Error(), http.StatusInternalServerError)
	}
	chore, choreCreateErr := models.ChoreFromForm(r.PostForm)
	if choreCreateErr != nil {
		http.Error(w, choreCreateErr.Error(), http.StatusInternalServerError)
	}

	db.Create(&chore)
	http.Redirect(w, r, routes.URLFor("choresDetail", chore.ID), http.StatusFound)
}
func ChoresDetail(w http.ResponseWriter, r *http.Request) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	choreID := chi.URLParam(r, "choreID")
	var chore models.Chore
	queryResult := db.First(&chore, choreID)
	if queryResult.Error != nil {
		http.Error(w, queryResult.Error.Error(), http.StatusInternalServerError)
	}
	component := choresDetailPage(chore)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}
}
