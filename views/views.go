package views

import (
	"context"
	"github.com/go-chi/chi/v5"
	"goDoChores/models"
	"goDoChores/routes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"net/http"
	"time"
)

func Home(w http.ResponseWriter, r *http.Request) {
	reminders, reminderListErr := getChoreReminderList()
	if reminderListErr != nil {
		http.Error(w, reminderListErr.Error(), http.StatusInternalServerError)
	}
	component := home(reminders)
	err := component.Render(context.Background(), w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func HomePost(w http.ResponseWriter, r *http.Request) {
	parseErr := r.ParseForm()
	if parseErr != nil {
		http.Error(w, parseErr.Error(), http.StatusInternalServerError)
	}
	log.Printf("%v", r.PostForm)
	reminderID := r.PostForm.Get("reminderID")
	if reminderID == "" {
		http.Error(w, "ReminderID required", http.StatusBadRequest)
		return
	}

	db, dbErr := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if dbErr != nil {
		http.Error(w, dbErr.Error(), http.StatusInternalServerError)
		return
	}

	var reminder models.ChoreReminder
	reminderQueryResult := db.First(&reminder, reminderID)
	if reminderQueryResult.Error != nil {
		http.Error(w, reminderQueryResult.Error.Error(), http.StatusInternalServerError)
		return
	}
	action := r.PostForm.Get("action")
	switch action {
	case "delete":
		db.Delete(&reminder, reminderID)
		break
	case "done":
		if reminder.Interval == "once" {
			db.Delete(&reminder, reminderID)
		} else {
			newDate, err := models.GetNextReminderDate(reminder)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			} else {
				reminder.Date = newDate
				db.Save(&reminder)
			}
		}
		break
	}

	http.Redirect(w, r, routes.URLFor("home"), http.StatusFound)
	return

}

func getAllChores() ([]models.Chore, error) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	var chores []models.Chore
	db.Find(&chores)

	return chores, nil
}

func ChoresList(w http.ResponseWriter, r *http.Request) {
	chores, err := getAllChores()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

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

type ChoreReminderListEntry struct {
	ReminderID int
	Interval   string
	Date       time.Time
	ChoreID    int
	Name       string
}

func getChoreReminderList() ([]ChoreReminderListEntry, error) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	var reminders []ChoreReminderListEntry
	db.Model(&models.ChoreReminder{}).Select("chore_reminders.id as reminder_id, chore_reminders.interval,chore_reminders.date, chores.id as chore_id, chores.name").Joins("left join chores on chore_reminders.chore_id = chores.id").Scan(&reminders)
	return reminders, nil
}

func RemindersList(w http.ResponseWriter, r *http.Request) {
	reminders, err := getChoreReminderList()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	component := remindersList(reminders)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}

}

func RemindersCreateGet(w http.ResponseWriter, r *http.Request) {
	chores, err := getAllChores()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	component := remindersCreatePage(chores, models.ValidIntervals)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}

}
func RemindersCreatePost(w http.ResponseWriter, r *http.Request) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	parseError := r.ParseForm()
	if parseError != nil {
		http.Error(w, parseError.Error(), http.StatusInternalServerError)
	}
	choreReminder, choreCreateErr := models.ChoreReminderFromForm(r.PostForm)
	if choreCreateErr != nil {
		http.Error(w, choreCreateErr.Error(), http.StatusInternalServerError)
	}

	db.Create(&choreReminder)
	http.Redirect(w, r, routes.URLFor("remindersDetail", choreReminder.ID), http.StatusFound)
}
func RemindersDetail(w http.ResponseWriter, r *http.Request) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reminderID := chi.URLParam(r, "reminderID")
	var reminder models.ChoreReminder
	reminderQueryResult := db.First(&reminder, reminderID)
	if reminderQueryResult.Error != nil {
		http.Error(w, reminderQueryResult.Error.Error(), http.StatusInternalServerError)
	}

	var chore models.Chore
	choreQueryResult := db.First(&chore, reminder.ChoreID)
	if choreQueryResult.Error != nil {
		http.Error(w, choreQueryResult.Error.Error(), http.StatusInternalServerError)
	}

	component := remindersDetailPage(chore, reminder)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}
}
func RemindersEditGet(w http.ResponseWriter, r *http.Request) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	chores, err := getAllChores()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	reminderID := chi.URLParam(r, "reminderID")
	var reminder models.ChoreReminder
	reminderQueryResult := db.First(&reminder, reminderID)
	if reminderQueryResult.Error != nil {
		http.Error(w, reminderQueryResult.Error.Error(), http.StatusInternalServerError)
	}
	component := remindersEditPage(reminder, chores, models.ValidIntervals)
	renderErr := component.Render(context.Background(), w)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}

}
func RemindersEditPost(w http.ResponseWriter, r *http.Request) {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	parseError := r.ParseForm()
	if parseError != nil {
		http.Error(w, parseError.Error(), http.StatusInternalServerError)
	}
	choreReminder, choreCreateErr := models.ChoreReminderFromForm(r.PostForm)
	if choreCreateErr != nil {
		http.Error(w, choreCreateErr.Error(), http.StatusInternalServerError)
	}

	db.Create(&choreReminder)
	http.Redirect(w, r, routes.URLFor("remindersDetail", choreReminder.ID), http.StatusFound)
}
