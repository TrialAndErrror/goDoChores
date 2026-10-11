package models

import (
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Chore struct {
	gorm.Model
	Name, Description string
	Time              string // freeform, e.g. "20 min" or "about an hour"
	Reminders         []ChoreReminder `gorm:"foreignKey:ChoreID"`
	Tags              []ChoreTag      `gorm:"foreignKey:ChoreID"`

	UserID uint
	User   User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

func ChoreFromForm(data url.Values, userID uint) (Chore, error) {
	name := strings.TrimSpace(data.Get("name"))
	if name == "" {
		return Chore{}, errors.New("name is required")
	}

	timeValue := strings.TrimSpace(data.Get("time"))
	if len(timeValue) > 100 {
		return Chore{}, errors.New("time must be 100 characters or fewer")
	}

	chore := Chore{
		Name:        name,
		Description: data.Get("description"),
		Time:        timeValue,
		UserID:      userID,
	}
	return chore, nil
}

type ChoreReminder struct {
	gorm.Model
	ChoreID  uint64
	Date     time.Time
	Interval string

	UserID uint
	User   User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// intervals is the single, ordered list of valid reminder intervals.
var intervals = []string{"Daily", "Weekly", "Monthly", "Annual", "Once"}

// GetIntervalNames returns a copy of the valid interval names, in display order.
func GetIntervalNames() []string {
	return slices.Clone(intervals)
}

// IsValidInterval reports whether name is a known reminder interval.
func IsValidInterval(name string) bool {
	return slices.Contains(intervals, name)
}

func ChoreReminderFromForm(data url.Values, userID uint) (ChoreReminder, error) {
	dateString := data.Get("date")
	dateFormatString := "2006-01-02"
	date, dateParseErr := time.Parse(dateFormatString, dateString)
	if dateParseErr != nil {
		return ChoreReminder{}, dateParseErr
	}

	choreIDString := data.Get("choreID")
	choreID, choreIDParseErr := strconv.ParseUint(choreIDString, 10, 64)
	if choreIDParseErr != nil {
		return ChoreReminder{}, choreIDParseErr
	}

	interval := data.Get("interval")
	if !IsValidInterval(interval) {
		return ChoreReminder{}, errors.New("invalid interval")
	}

	reminder := ChoreReminder{
		ChoreID:  choreID,
		Date:     date,
		Interval: interval,
		UserID:   userID,
	}
	return reminder, nil

}

func GetNextReminderDate(reminder ChoreReminder) (time.Time, error) {
	switch reminder.Interval {
	case "Daily":
		return time.Now().AddDate(0, 0, 1), nil
	case "Weekly":
		return time.Now().AddDate(0, 0, 7), nil
	case "Monthly":
		return time.Now().AddDate(0, 1, 0), nil
	case "Annual":
		return time.Now().AddDate(1, 0, 0), nil
	default:
		return time.Time{}, errors.New("invalid interval")
	}
}

type User struct {
	gorm.Model
	Username     string `gorm:"uniqueIndex;not null"`
	Email        string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`

	// SessionVersion is stamped into every login token. Bumping it (on logout
	// or a password change) invalidates all tokens issued before.
	SessionVersion uint `gorm:"not null;default:0"`

	Chores         []Chore
	ChoreReminders []ChoreReminder
	Tasks          []Task
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

type Task struct {
	gorm.Model
	Name        string
	Description string
	Date        time.Time

	UserID uint
	User   User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

func TaskFromForm(data url.Values, userID uint) (Task, error) {
	name := strings.TrimSpace(data.Get("name"))
	if name == "" {
		return Task{}, errors.New("name is required")
	}

	dateString := data.Get("date")
	dateFormatString := "2006-01-02"
	date, dateParseErr := time.Parse(dateFormatString, dateString)
	if dateParseErr != nil {
		return Task{}, dateParseErr
	}

	task := Task{
		Name:        name,
		Description: data.Get("description"),
		Date:        date,
		UserID:      userID,
	}
	return task, nil
}

// TagNames returns the chore's tag names, sorted. Load "Tags.Tag" first.
func (c Chore) TagNames() []string {
	names := make([]string, 0, len(c.Tags))
	for _, t := range c.Tags {
		names = append(names, t.Tag.Name)
	}
	slices.Sort(names)
	return names
}
