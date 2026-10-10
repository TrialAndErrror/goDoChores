// Package cli holds the administration commands: managing users, backing up and
// restoring the database, and the container health check.
package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"goDoChores/models"

	"gorm.io/gorm"
)

// UserSummary is a user plus how much data they own.
type UserSummary struct {
	ID        uint
	Username  string
	Email     string
	CreatedAt time.Time
	Chores    int64
	Reminders int64
	Tasks     int64
}

func summarize(db *gorm.DB, u models.User) (UserSummary, error) {
	s := UserSummary{ID: u.ID, Username: u.Username, Email: u.Email, CreatedAt: u.CreatedAt}
	for model, dest := range map[any]*int64{
		&models.Chore{}:         &s.Chores,
		&models.ChoreReminder{}: &s.Reminders,
		&models.Task{}:          &s.Tasks,
	} {
		if err := db.Model(model).Where("user_id = ?", u.ID).Count(dest).Error; err != nil {
			return s, err
		}
	}
	return s, nil
}

// SaveUser creates the user, or updates the email and password of an existing
// user with the same username. It reports whether a new user was created.
func SaveUser(db *gorm.DB, username, email, password string) (created bool, err error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	switch {
	case username == "":
		return false, errors.New("username is required")
	case email == "":
		return false, errors.New("email is required")
	case password == "":
		return false, errors.New("password is required")
	}

	var user models.User
	findErr := db.Where("username = ?", username).First(&user).Error
	switch {
	case findErr == nil:
		user.Email = email
		// A new password must sign the user out everywhere.
		user.SessionVersion++
	case errors.Is(findErr, gorm.ErrRecordNotFound):
		created = true
		user = models.User{Username: username, Email: email}
	default:
		return false, fmt.Errorf("looking up user: %w", findErr)
	}

	if err := user.SetPassword(password); err != nil {
		return false, fmt.Errorf("hashing password: %w", err)
	}
	if err := db.Save(&user).Error; err != nil {
		return false, fmt.Errorf("saving user: %w", err)
	}
	return created, nil
}

// ListUsers returns every user, oldest first, with their data counts.
func ListUsers(db *gorm.DB) ([]UserSummary, error) {
	var users []models.User
	if err := db.Order("id").Find(&users).Error; err != nil {
		return nil, err
	}
	out := make([]UserSummary, 0, len(users))
	for _, u := range users {
		s, err := summarize(db, u)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// DescribeUser returns one user by username, or an error if there is none.
func DescribeUser(db *gorm.DB, username string) (UserSummary, error) {
	var user models.User
	err := db.Where("username = ?", strings.TrimSpace(username)).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return UserSummary{}, fmt.Errorf("no user named %q", username)
	}
	if err != nil {
		return UserSummary{}, err
	}
	return summarize(db, user)
}

// DeleteUser permanently removes a user and all of their chores, reminders and
// tasks in one transaction. It is a hard delete on purpose: a soft-deleted row
// would keep its username and email in the unique indexes, so the same name
// could never be used again.
func DeleteUser(db *gorm.DB, username string) (UserSummary, error) {
	summary, err := DescribeUser(db, username)
	if err != nil {
		return summary, err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, model := range []any{&models.ChoreReminder{}, &models.Task{}, &models.Chore{}} {
			if err := tx.Unscoped().Where("user_id = ?", summary.ID).Delete(model).Error; err != nil {
				return err
			}
		}
		return tx.Unscoped().Delete(&models.User{}, summary.ID).Error
	})
	if err != nil {
		return summary, fmt.Errorf("deleting user: %w", err)
	}
	return summary, nil
}
