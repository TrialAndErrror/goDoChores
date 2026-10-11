package models

import (
	"net/url"
	"testing"
	"time"
)

func TestChoreFromForm(t *testing.T) {
	chore, err := ChoreFromForm(url.Values{
		"name":        {"Dishes"},
		"description": {"Wash up"},
		"time":        {"15"},
	}, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chore.Name != "Dishes" || chore.Description != "Wash up" || chore.Time != 15 || chore.UserID != 7 {
		t.Errorf("unexpected chore: %+v", chore)
	}

	// Surrounding whitespace is trimmed from the name.
	trimmed, err := ChoreFromForm(url.Values{"name": {"  Dishes  "}, "time": {"0"}}, 1)
	if err != nil || trimmed.Name != "Dishes" || trimmed.Time != 0 {
		t.Errorf("trim/zero time: got %+v, %v", trimmed, err)
	}

	for _, name := range []string{"", "   "} {
		if _, err := ChoreFromForm(url.Values{"name": {name}, "time": {"5"}}, 1); err == nil {
			t.Errorf("name %q: expected error", name)
		}
	}
	if _, err := ChoreFromForm(url.Values{"name": {"x"}, "time": {"-5"}}, 1); err == nil {
		t.Error("negative time: expected error")
	}

	// A blank or missing time is optional and stored as 0.
	for _, form := range []url.Values{{"name": {"x"}, "time": {""}}, {"name": {"x"}}, {"name": {"x"}, "time": {"  "}}} {
		blank, err := ChoreFromForm(form, 1)
		if err != nil || blank.Time != 0 {
			t.Errorf("blank time %v: got %+v, %v", form, blank, err)
		}
	}

	for _, bad := range []string{"abc", "1.5"} {
		if _, err := ChoreFromForm(url.Values{"name": {"x"}, "time": {bad}}, 1); err == nil {
			t.Errorf("time %q: expected error", bad)
		}
	}
}

func TestChoreReminderFromForm(t *testing.T) {
	valid := func() url.Values {
		return url.Values{"date": {"2026-10-12"}, "choreID": {"3"}, "interval": {"Weekly"}}
	}

	r, err := ChoreReminderFromForm(valid(), 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	if !r.Date.Equal(want) || r.ChoreID != 3 || r.Interval != "Weekly" || r.UserID != 9 {
		t.Errorf("unexpected reminder: %+v", r)
	}

	cases := map[string]func(url.Values){
		"bad date":         func(v url.Values) { v.Set("date", "12/10/2026") },
		"missing date":     func(v url.Values) { v.Del("date") },
		"bad chore id":     func(v url.Values) { v.Set("choreID", "x") },
		"negative chore":   func(v url.Values) { v.Set("choreID", "-1") },
		"unknown interval": func(v url.Values) { v.Set("interval", "Hourly") },
		"missing interval": func(v url.Values) { v.Del("interval") },
	}
	for name, mutate := range cases {
		v := valid()
		mutate(v)
		if _, err := ChoreReminderFromForm(v, 1); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestTaskFromForm(t *testing.T) {
	task, err := TaskFromForm(url.Values{
		"name": {"Pay rent"}, "description": {"by Friday"}, "date": {"2026-11-01"},
	}, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Name != "Pay rent" || task.Description != "by Friday" || task.UserID != 4 ||
		!task.Date.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("unexpected task: %+v", task)
	}

	if _, err := TaskFromForm(url.Values{"name": {"x"}, "date": {"nope"}}, 1); err == nil {
		t.Error("expected error for bad date")
	}
	for _, name := range []string{"", "  "} {
		if _, err := TaskFromForm(url.Values{"name": {name}, "date": {"2026-11-01"}}, 1); err == nil {
			t.Errorf("name %q: expected error", name)
		}
	}
}

func TestIntervals(t *testing.T) {
	names := GetIntervalNames()
	if len(names) == 0 {
		t.Fatal("no intervals")
	}
	for _, n := range names {
		if !IsValidInterval(n) {
			t.Errorf("%q listed but not valid", n)
		}
	}
	if IsValidInterval("") || IsValidInterval("daily") || IsValidInterval("Hourly") {
		t.Error("invalid interval accepted")
	}

	// Mutating the returned slice must not change the package's list.
	names[0] = "Tampered"
	if GetIntervalNames()[0] == "Tampered" {
		t.Error("GetIntervalNames exposes internal slice")
	}
}

func TestGetNextReminderDate(t *testing.T) {
	now := time.Now()
	cases := map[string]time.Time{
		"Daily":   now.AddDate(0, 0, 1),
		"Weekly":  now.AddDate(0, 0, 7),
		"Monthly": now.AddDate(0, 1, 0),
		"Annual":  now.AddDate(1, 0, 0),
	}
	for interval, want := range cases {
		got, err := GetNextReminderDate(ChoreReminder{Interval: interval})
		if err != nil {
			t.Errorf("%s: unexpected error: %v", interval, err)
			continue
		}
		if d := got.Sub(want); d < -time.Minute || d > time.Minute {
			t.Errorf("%s: got %v, want about %v", interval, got, want)
		}
	}

	// "Once" reminders are deleted by callers, so they have no next date.
	for _, interval := range []string{"Once", "", "Bogus"} {
		if _, err := GetNextReminderDate(ChoreReminder{Interval: interval}); err == nil {
			t.Errorf("%q: expected error", interval)
		}
	}
}

func TestPasswordHashing(t *testing.T) {
	var u User
	if err := u.SetPassword("hunter2"); err != nil {
		t.Fatal(err)
	}
	if u.PasswordHash == "" || u.PasswordHash == "hunter2" {
		t.Errorf("password not hashed: %q", u.PasswordHash)
	}
	if !u.CheckPassword("hunter2") {
		t.Error("correct password rejected")
	}
	if u.CheckPassword("hunter3") || u.CheckPassword("") {
		t.Error("wrong password accepted")
	}
}
