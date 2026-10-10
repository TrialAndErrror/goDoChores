package routes

import "testing"

func TestURLFor(t *testing.T) {
	if got := URLFor("home"); got != "/" {
		t.Errorf("home: got %q", got)
	}
	if got := URLFor("choresDetail", 12); got != "/chores/12/" {
		t.Errorf("choresDetail: got %q", got)
	}
	if got := URLFor("tasksEdit", uint(3)); got != "/tasks/3/edit" {
		t.Errorf("tasksEdit: got %q", got)
	}
}

func TestURLForUnknownRoutePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for unknown route")
		}
	}()
	URLFor("nope")
}
