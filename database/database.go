// Package database owns the single shared GORM handle used by the whole app.
package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"goDoChores/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
	// "Record not found" is an ordinary outcome (an unknown ID, a username that
	// is free), not something to log with a stack of SQL every time.
	quiet := logger.New(log.New(os.Stderr, "\r\n", log.LstdFlags), logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
	})
	db, err := gorm.Open(sqlite.Open("file:"+Path()+"?_busy_timeout=5000"), &gorm.Config{Logger: quiet})
	if err != nil {
		return fmt.Errorf("opening database %q: %w", Path(), err)
	}
	DB = db
	return nil
}

// Close releases the database connection. Call it once, after the server has
// stopped handling requests.
func Close() error {
	if DB == nil {
		return nil
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Migrate creates or updates every table the app uses. Both the server and the
// CLI call it after Open so a fresh database always has its schema.
func Migrate() error {
	if DB == nil {
		return fmt.Errorf("database is not open")
	}
	if err := DB.AutoMigrate(&models.Chore{}, &models.ChoreReminder{}, &models.Task{}, &models.User{}); err != nil {
		return fmt.Errorf("migrating database: %w", err)
	}
	return nil
}
