package database

import (
	"path/filepath"
	"testing"
	"time"

	"goDoChores/models"
)

func TestMigrateRewritesLegacyIntervals(t *testing.T) {
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "t.db"))
	if err := Open(); err != nil {
		t.Fatal(err)
	}
	defer Close()
	if err := Migrate(); err != nil {
		t.Fatal(err)
	}
	for _, old := range []string{"Daily", "Weekly", "Monthly", "Annual", "Once"} {
		DB.Create(&models.ChoreReminder{Interval: old, Date: time.Now()})
	}
	if err := Migrate(); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"Day": 1, "Week": 1, "Month": 1, "Year": 1, "Once": 0}
	var got []models.ChoreReminder
	DB.Find(&got)
	if len(got) != len(want) {
		t.Fatalf("got %d reminders", len(got))
	}
	for _, r := range got {
		if v, ok := want[r.Interval]; !ok || v != r.IntervalValue {
			t.Errorf("unexpected %q x%d", r.Interval, r.IntervalValue)
		}
	}
}
