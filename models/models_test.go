package models

import (
	"net/url"
	"strings"
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
	if chore.Name != "Dishes" || chore.Description != "Wash up" || chore.Time != "15" || chore.UserID != 7 {
		t.Errorf("unexpected chore: %+v", chore)
	}

	// Surrounding whitespace is trimmed from the name.
	trimmed, err := ChoreFromForm(url.Values{"name": {"  Dishes  "}, "time": {"0"}}, 1)
	if err != nil || trimmed.Name != "Dishes" || trimmed.Time != "0" {
		t.Errorf("trim/zero time: got %+v, %v", trimmed, err)
	}

	for _, name := range []string{"", "   "} {
		if _, err := ChoreFromForm(url.Values{"name": {name}, "time": {"5"}}, 1); err == nil {
			t.Errorf("name %q: expected error", name)
		}
	}
	// Time is freeform text: blank is fine, anything reasonable is kept as typed.
	for in, want := range map[string]string{"": "", "  ": "", " about an hour ": "about an hour", "20 min": "20 min"} {
		c, err := ChoreFromForm(url.Values{"name": {"x"}, "time": {in}}, 1)
		if err != nil || c.Time != want {
			t.Errorf("time %q: got %q, %v; want %q", in, c.Time, err, want)
		}
	}
	if _, err := ChoreFromForm(url.Values{"name": {"x"}, "time": {strings.Repeat("a", 101)}}, 1); err == nil {
		t.Error("overlong time: expected error")
	}
}

func TestChoreReminderFromForm(t *testing.T) {
	valid := func() url.Values {
		return url.Values{"date": {"2026-10-12"}, "choreID": {"3"}, "interval": {"Week"}, "intervalValue": {"1"}}
	}

	r, err := ChoreReminderFromForm(valid(), 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	if !r.Date.Equal(want) || r.ChoreID != 3 || r.Interval != "Week" || r.UserID != 9 {
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
	day := func(offset int) time.Time {
		y, m, d := now.AddDate(0, 0, offset).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	cases := []struct {
		name     string
		interval string
		value    int
		due      time.Time
		want     time.Time
	}{
		{"due today", "Day", 1, day(0), day(1)},
		{"due tomorrow is skipped", "Day", 1, day(1), day(2)},
		{"3 days late, every 7 days stays anchored", "Week", 1, day(-3), day(4)},
		{"every 2 weeks, due today", "Week", 2, day(0), day(14)},
		{"long overdue skips missed occurrences", "Day", 3, day(-10), day(2)},
		{"early completion", "Week", 1, day(5), day(12)},
	}
	for _, c := range cases {
		got, err := GetNextReminderDate(ChoreReminder{Interval: c.interval, IntervalValue: c.value, Date: c.due})
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}

	// A day-of-month reminder lands on that day, after its current date.
	got, err := GetNextReminderDate(ChoreReminder{Interval: "DOM", IntervalValue: 26, Date: now.AddDate(0, 1, 0)})
	if err != nil || got.Day() != min(26, clampDay(got.Year(), got.Month(), 26)) || !got.After(now) {
		t.Errorf("DOM: got %v, %v", got, err)
	}

	// "Once" reminders are deleted by callers, so they have no next date.
	for _, interval := range []string{"Once", "", "Bogus"} {
		if _, err := GetNextReminderDate(ChoreReminder{Interval: interval, IntervalValue: 1}); err == nil {
			t.Errorf("%q: expected error", interval)
		}
	}
	// A repeating reminder with no count would otherwise schedule for today.
	if _, err := GetNextReminderDate(ChoreReminder{Interval: "Week"}); err == nil {
		t.Error("zero interval value: expected error")
	}
}

func TestDescribe(t *testing.T) {
	cases := map[string]ChoreReminder{
		"Every day":                  {Interval: "Day", IntervalValue: 1},
		"Every 2 weeks":              {Interval: "Week", IntervalValue: 2},
		"Every year":                 {Interval: "Year", IntervalValue: 1},
		"On the 1st of every month":  {Interval: "DOM", IntervalValue: 1},
		"On the 22nd of every month": {Interval: "DOM", IntervalValue: 22},
		"On the 11th of every month": {Interval: "DOM", IntervalValue: 11},
		"On the 26th of every month": {Interval: "DOM", IntervalValue: 26},
		"Once":                       {Interval: "Once"},
	}
	for want, r := range cases {
		if got := r.Describe(); got != want {
			t.Errorf("%+v: got %q, want %q", r, got, want)
		}
	}
}

func TestReminderIntervalValueFromForm(t *testing.T) {
	form := func(interval, value string) url.Values {
		return url.Values{"date": {"2026-10-12"}, "choreID": {"3"}, "interval": {interval}, "intervalValue": {value}}
	}
	for _, c := range []struct {
		interval, value string
		want            int
		ok              bool
	}{
		{"Week", "2", 2, true},
		{"DOM", "31", 31, true},
		{"Once", "", 0, true},
		{"Once", "7", 0, true},
		{"Week", "", 0, false},
		{"Week", "0", 0, false},
		{"Week", "-1", 0, false},
		{"Week", "x", 0, false},
		{"DOM", "32", 0, false},
	} {
		r, err := ChoreReminderFromForm(form(c.interval, c.value), 1)
		if (err == nil) != c.ok || (c.ok && r.IntervalValue != c.want) {
			t.Errorf("%s %q: got %d, %v", c.interval, c.value, r.IntervalValue, err)
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

func TestNextDayOfMonth(t *testing.T) {
	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.Local) }
	cases := []struct {
		name     string
		due, now time.Time
		day      int
		want     time.Time
	}{
		{"done early, before due", d(2026, 1, 26), d(2026, 1, 10), 26, d(2026, 2, 26)},
		{"done late, past due", d(2026, 1, 26), d(2026, 2, 10), 26, d(2026, 2, 26)},
		{"done on the due day", d(2026, 1, 26), d(2026, 1, 26), 26, d(2026, 2, 26)},
		{"done late, past this month's day", d(2026, 1, 26), d(2026, 2, 27), 26, d(2026, 3, 26)},
		{"december rolls the year", d(2026, 12, 26), d(2026, 12, 10), 26, d(2027, 1, 26)},
		{"31st clamps to short month", d(2026, 1, 31), d(2026, 1, 15), 31, d(2026, 2, 28)},
		{"31st after clamped month", d(2026, 2, 28), d(2026, 2, 28), 31, d(2026, 3, 31)},
	}
	for _, c := range cases {
		if got := nextDayOfMonth(c.due, c.now, c.day); !got.Equal(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestNextAfterMonthsAndYears(t *testing.T) {
	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }
	now := d(2026, 3, 10)
	cases := []struct {
		name string
		step func(t time.Time, k int) time.Time
		due  time.Time
		want time.Time
	}{
		{"month clamps Jan 31", func(t time.Time, k int) time.Time { return addMonthsClamped(t, k) }, d(2026, 1, 31), d(2026, 3, 31)},
		{"month from Feb 28", func(t time.Time, k int) time.Time { return addMonthsClamped(t, k) }, d(2026, 2, 28), d(2026, 3, 28)},
		{"every 2 months, due Jan 15", func(t time.Time, k int) time.Time { return addMonthsClamped(t, 2*k) }, d(2026, 1, 15), d(2026, 3, 15)},
		{"month rolls the year", func(t time.Time, k int) time.Time { return addMonthsClamped(t, k) }, d(2026, 12, 20), d(2027, 1, 20)},
		{"year clamps Feb 29", func(t time.Time, k int) time.Time { return addMonthsClamped(t, 12*k) }, d(2024, 2, 29), d(2027, 2, 28)},
	}
	for _, c := range cases {
		nw := now
		if c.name == "month rolls the year" {
			nw = d(2026, 12, 25)
		}
		if got := nextAfter(c.due, nw, c.step); !got.Equal(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
