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
	Time              string          // freeform, e.g. "20 min" or "about an hour"
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
	// IntervalValue is the count for Day/Week/Month/Year ("every N weeks") and
	// the day of the month for DOM. It is 0 for Once.
	IntervalValue int `gorm:"not null;default:0"`

	UserID uint
	User   User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// intervals is the single, ordered list of valid reminder intervals.
var intervals = []string{"Day", "Week", "Month", "Year", "DOM", "Once"}

// intervalLabels is how each interval is named in the reminder form.
var intervalLabels = map[string]string{
	"Day":   "Day(s)",
	"Week":  "Week(s)",
	"Month": "Month(s)",
	"Year":  "Year(s)",
	"DOM":   "Day of the month",
	"Once":  "Once",
}

// IntervalLabel returns the form label for an interval name.
func IntervalLabel(name string) string {
	if l, ok := intervalLabels[name]; ok {
		return l
	}
	return name
}

// Describe returns the reminder's schedule in words, e.g. "Every 2 weeks" or
// "On the 26th of every month".
func (r ChoreReminder) Describe() string {
	n := r.IntervalValue
	plural := func(unit string) string {
		if n == 1 {
			return "Every " + unit
		}
		return "Every " + strconv.Itoa(n) + " " + unit + "s"
	}
	switch r.Interval {
	case "Day", "Week", "Month", "Year":
		return plural(strings.ToLower(r.Interval))
	case "DOM":
		return "On the " + ordinal(n) + " of every month"
	case "Once":
		return "Once"
	}
	return r.Interval
}

func ordinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return strconv.Itoa(n) + suffix
}

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

	intervalValue := 0
	if interval != "Once" {
		v, err := strconv.Atoi(strings.TrimSpace(data.Get("intervalValue")))
		if err != nil || v < 1 {
			return ChoreReminder{}, errors.New("interval value must be a whole number of at least 1")
		}
		if interval == "DOM" && v > 31 {
			return ChoreReminder{}, errors.New("day of the month must be between 1 and 31")
		}
		intervalValue = v
	}

	reminder := ChoreReminder{
		ChoreID:       choreID,
		Date:          date,
		Interval:      interval,
		IntervalValue: intervalValue,
		UserID:        userID,
	}
	return reminder, nil
}

func GetNextReminderDate(reminder ChoreReminder) (time.Time, error) {
	if reminder.Interval != "Once" && reminder.IntervalValue < 1 {
		return time.Time{}, errors.New("invalid interval value")
	}
	now := time.Now()
	n := reminder.IntervalValue
	switch reminder.Interval {
	case "Day":
		return nextAfter(reminder.Date, now, func(t time.Time, k int) time.Time { return t.AddDate(0, 0, k*n) }), nil
	case "Week":
		return nextAfter(reminder.Date, now, func(t time.Time, k int) time.Time { return t.AddDate(0, 0, 7*k*n) }), nil
	case "Month":
		return nextAfter(reminder.Date, now, func(t time.Time, k int) time.Time { return addMonthsClamped(t, k*n) }), nil
	case "Year":
		return nextAfter(reminder.Date, now, func(t time.Time, k int) time.Time { return addMonthsClamped(t, 12*k*n) }), nil
	case "DOM":
		return nextDayOfMonth(reminder.Date, now, reminder.IntervalValue), nil
	default:
		return time.Time{}, errors.New("invalid interval")
	}
}

// nextAfter steps due forward by step(due, 1), step(due, 2), ... and returns
// the first date that falls on a calendar day after today. The schedule stays
// anchored to the reminder's own date: completing it late (or early) never
// shifts it, and a long-overdue reminder skips the occurrences already missed.
func nextAfter(due, now time.Time, step func(t time.Time, k int) time.Time) time.Time {
	for k := 1; ; k++ {
		next := step(due, k)
		if dayKey(next) > dayKey(now) {
			return next
		}
	}
}

// dayKey orders times by calendar day, ignoring time of day and location.
func dayKey(t time.Time) int {
	y, m, d := t.Date()
	return y*10000 + int(m)*100 + d
}

// addMonthsClamped adds months to t, clamping to the last day of a shorter
// target month (Jan 31 + 1 month is Feb 28, not Mar 3).
func addMonthsClamped(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	m += time.Month(months)
	// Normalize the month into the year before clamping the day.
	first := time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
	return time.Date(first.Year(), first.Month(), clampDay(first.Year(), first.Month(), d),
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

// nextDayOfMonth returns the first occurrence of the given day of the month
// strictly after the later of due and now. Completing a reminder early
// (before due) therefore skips the occurrence being completed, while
// completing it late lands on the next occurrence still ahead of today.
// Days past the end of a short month are clamped to its last day.
func nextDayOfMonth(due, now time.Time, day int) time.Time {
	base := now
	if due.After(now) {
		base = due
	}
	y, m, d := base.Date()
	if d >= clampDay(y, m, day) {
		m++ // time.Date normalizes month 13 into January of the next year
	}
	return time.Date(y, m, clampDay(y, m, day), 0, 0, 0, 0, time.Local)
}

// clampDay limits day to the number of days in the given month.
func clampDay(year int, month time.Month, day int) int {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return min(day, last)
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
