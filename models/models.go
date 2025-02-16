package models

import (
	"errors"
	"goDoChores/utils"
	"gorm.io/gorm"
	"net/url"
	"strconv"
	"time"
)

type Chore struct {
	gorm.Model
	Name, Description string
	Time              int
	Reminders         []ChoreReminder `gorm:"foreignKey:ChoreID"`
}

func ChoreFromForm(data url.Values) (Chore, error) {
	timeString := data.Get("time")
	timeValue, timeParseErr := strconv.Atoi(timeString)
	if timeParseErr != nil {
		return Chore{}, timeParseErr
	}

	chore := Chore{
		Name:        data.Get("name"),
		Description: data.Get("description"),
		Time:        timeValue,
	}
	return chore, nil
}

type ChoreReminder struct {
	gorm.Model
	ChoreID  uint64
	Date     time.Time
	Interval string
}

var ValidIntervals = map[string]string{
	"Daily":   "day",
	"Weekly":  "week",
	"Monthly": "month",
	"Annual":  "year",
	"Once":    "once",
}

var IntervalNames = utils.ReverseMap(ValidIntervals)

func ChoreReminderFromForm(data url.Values) (ChoreReminder, error) {
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
	_, intervalOk := IntervalNames[interval]
	if !intervalOk {
		return ChoreReminder{}, errors.New("invalid interval")
	}

	reminder := ChoreReminder{
		ChoreID:  choreID,
		Date:     date,
		Interval: interval,
	}
	return reminder, nil

}

func GetNextReminderDate(reminder ChoreReminder) (newDate time.Time, error error) {
	switch reminder.Interval {
	case "day":
		return reminder.Date.AddDate(0, 0, 1), nil
	case "week":
		return reminder.Date.AddDate(0, 0, 7), nil
	case "month":
		return reminder.Date.AddDate(0, 1, 0), nil
	case "year":
		return reminder.Date.AddDate(1, 0, 0), nil
	default:
		return time.Time{}, errors.New("invalid interval")
	}
}
