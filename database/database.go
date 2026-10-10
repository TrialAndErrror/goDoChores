// Package database owns the single shared GORM handle used by the whole app.
package database

import (
	"fmt"
	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// DB is the shared database handle. It is set by Open and must not be used
// before Open has been called.
var DB *gorm.DB

// Path returns the SQLite file to use: DB_PATH if set, otherwise test.db.
func Path() string {
	if p := os.Getenv("DB_PATH"); p != "" {
		return p
	}
	return "test.db"
}

// Open connects to the SQLite database once and stores it in DB.
func Open() error {
	// busy_timeout makes concurrent writers wait instead of failing with
	// "database is locked".
	db, err := gorm.Open(sqlite.Open("file:"+Path()+"?_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("opening database %q: %w", Path(), err)
	}
	DB = db
	return nil
}
