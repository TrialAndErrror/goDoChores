package models

import "gorm.io/gorm"

type Chore struct {
	gorm.Model
	Name        string
	Description string
	Time        int
}
