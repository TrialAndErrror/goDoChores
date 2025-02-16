package models

import (
	"gorm.io/gorm"
	"net/url"
	"strconv"
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
