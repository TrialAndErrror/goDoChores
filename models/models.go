package models

import (
	"gorm.io/gorm"
	"net/url"
	"strconv"
	"time"
)

type Chore struct {
	gorm.Model
	Name, Description string
	Time              int
}

func ChoreFromForm(data url.Values) (Chore, error) {
	timeString := data.Get("time")
	time, timeParseErr := strconv.Atoi(timeString)
	if timeParseErr != nil {
		return Chore{}, timeParseErr
	}

	chore := Chore{
		Name:        data.Get("name"),
		Description: data.Get("description"),
		Time:        time,
	}
	return chore, nil
}

type ChoreReminder struct {
	gorm.Model
	choreID  int
	date     time.Time
	interval string
}

func ChoreReminderFromForm(data url.Values) (ChoreReminder, error) {
	dateString := data.Get("time")
	dateFormatString := "2021-03-11"
	date, dateParseErr := time.Parse(dateString, dateFormatString)
	if dateParseErr != nil {
		return ChoreReminder{}, dateParseErr
	}

	choreIDString := data.Get("choreID")
	choreID, choreIDParseErr := strconv.Atoi(choreIDString)
	if choreIDParseErr != nil {
		return ChoreReminder{}, choreIDParseErr
	}

	interval := data.Get("interval")

	reminder := ChoreReminder{
		choreID:  choreID,
		date:     date,
		interval: interval,
	}
	return reminder, nil

}
