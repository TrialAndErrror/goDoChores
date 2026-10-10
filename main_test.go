package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"

	"github.com/go-chi/jwtauth/v5"
)

const testSecret = "test-secret"

type testApp struct {
	t       *testing.T
	h       http.Handler
	cookies map[string]*http.Cookie
}

// seed holds the IDs of the rows created for the two test users.
type seed struct {
	aliceChore, aliceReminder, aliceTask uint
	bobChore, bobReminder, bobTask       uint
}

func newTestApp(t *testing.T) (*testApp, seed) {
	t.Helper()
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "test.db"))
	if err := database.Open(); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.AutoMigrate(&models.Chore{}, &models.ChoreReminder{}, &models.Task{}, &models.User{}); err != nil {
		t.Fatal(err)
	}
	tokenAuth := auth.Init(testSecret)

	db := database.DB
	users := map[string]*models.User{
		"alice": {Username: "alice", Email: "alice@example.com"},
		"bob":   {Username: "bob", Email: "bob@example.com"},
	}
	for name, u := range users {
		if err := u.SetPassword(name + "-pw"); err != nil {
			t.Fatal(err)
		}
		if err := db.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}

	var s seed
	mk := func(u *models.User) (chore, reminder, task uint) {
		c := models.Chore{Name: u.Username + " chore", Description: "d", Time: 10, UserID: u.ID}
		db.Create(&c)
		r := models.ChoreReminder{ChoreID: uint64(c.ID), Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Interval: "Weekly", UserID: u.ID}
		db.Create(&r)
		tk := models.Task{Name: u.Username + " task", Description: "d", Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), UserID: u.ID}
		db.Create(&tk)
		return c.ID, r.ID, tk.ID
	}
	s.aliceChore, s.aliceReminder, s.aliceTask = mk(users["alice"])
	s.bobChore, s.bobReminder, s.bobTask = mk(users["bob"])

	return &testApp{t: t, h: newRouter(tokenAuth), cookies: map[string]*http.Cookie{}}, s
}

// do sends a request as the given user ("" = anonymous).
func (a *testApp) do(user, method, path string, form url.Values) *httptest.ResponseRecorder {
	a.t.Helper()
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if user != "" {
		req.AddCookie(a.login(user))
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	return rec
}

func (a *testApp) login(user string) *http.Cookie {
	a.t.Helper()
	if c, ok := a.cookies[user]; ok {
		return c
	}
	rec := a.postLogin(user, user+"-pw")
	for _, c := range rec.Result().Cookies() {
		if c.Name == "jwt" && c.Value != "" {
			a.cookies[user] = c
			return c
		}
	}
	a.t.Fatalf("login as %s failed: status %d", user, rec.Code)
	return nil
}

func (a *testApp) postLogin(user, pw string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/login", strings.NewReader(url.Values{"username": {user}, "password": {pw}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	return rec
}

func count(t *testing.T, model any, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := database.DB.Model(model).Where(query, args...).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func expectStatus(t *testing.T, rec *httptest.ResponseRecorder, want int, what string) {
	t.Helper()
	if rec.Code != want {
		t.Errorf("%s: got status %d, want %d (body: %.120q)", what, rec.Code, want, rec.Body.String())
	}
}

func TestUnauthenticatedRequestsAreRedirectedAndNeverReachHandlers(t *testing.T) {
	a, s := newTestApp(t)

	gets := []string{
		"/", "/chores/", "/chores/new", fmt.Sprintf("/chores/%d/", s.aliceChore), fmt.Sprintf("/chores/%d/edit", s.aliceChore),
		"/reminders/", "/reminders/new", fmt.Sprintf("/reminders/%d/", s.aliceReminder),
		"/tasks/", "/tasks/new", fmt.Sprintf("/tasks/%d/", s.aliceTask),
	}
	for _, p := range gets {
		rec := a.do("", "GET", p, nil)
		expectStatus(t, rec, http.StatusFound, "GET "+p)
		if loc := rec.Header().Get("Location"); loc != "/login" {
			t.Errorf("GET %s: redirect to %q, want /login", p, loc)
		}
		if strings.Contains(rec.Body.String(), "alice") {
			t.Errorf("GET %s: leaked data to anonymous user", p)
		}
	}

	// A POST must not run the handler behind the redirect.
	before := count(t, &models.Chore{}, "1 = 1")
	rec := a.do("", "POST", "/chores/new", url.Values{"name": {"sneaky"}, "description": {"d"}, "time": {"5"}})
	expectStatus(t, rec, http.StatusFound, "anonymous chore create")
	if after := count(t, &models.Chore{}, "1 = 1"); after != before {
		t.Errorf("anonymous POST created a chore (%d -> %d)", before, after)
	}

	rec = a.do("", "POST", fmt.Sprintf("/chores/%d/", s.aliceChore), url.Values{"action": {"delete"}})
	expectStatus(t, rec, http.StatusFound, "anonymous chore delete")
	if count(t, &models.Chore{}, "id = ?", s.aliceChore) != 1 {
		t.Error("anonymous POST deleted a chore")
	}
}

func TestInvalidTokensAreRejected(t *testing.T) {
	a, _ := newTestApp(t)

	_, forgedStr, _ := jwtauth.New("HS256", []byte("some-other-secret"), nil).Encode(map[string]interface{}{"user_id": 1})
	_, expiredStr, _ := auth.Init(testSecret).Encode(map[string]interface{}{
		"user_id": 1,
		"exp":     time.Now().Add(-time.Hour).Unix(),
	})

	for name, tok := range map[string]string{
		"wrong secret": forgedStr,
		"expired":      expiredStr,
		"garbage":      "not.a.jwt",
	} {
		req := httptest.NewRequest("GET", "/chores/", nil)
		req.AddCookie(&http.Cookie{Name: "jwt", Value: tok})
		rec := httptest.NewRecorder()
		a.h.ServeHTTP(rec, req)
		expectStatus(t, rec, http.StatusFound, name+" token")
	}
}

func TestLogin(t *testing.T) {
	a, _ := newTestApp(t)

	for name, creds := range map[string][2]string{
		"wrong password": {"alice", "nope"},
		"unknown user":   {"mallory", "x"},
		"empty":          {"", ""},
	} {
		rec := a.postLogin(creds[0], creds[1])
		expectStatus(t, rec, http.StatusUnauthorized, name)
		for _, c := range rec.Result().Cookies() {
			if c.Name == "jwt" {
				t.Errorf("%s: a jwt cookie was set on a failed login", name)
			}
		}
		if loc := rec.Header().Get("Location"); loc != "" {
			t.Errorf("%s: failed login redirected to %q", name, loc)
		}
	}

	rec := a.postLogin("alice", "alice-pw")
	expectStatus(t, rec, http.StatusSeeOther, "good login")
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "jwt" {
			cookie = c
		}
	}
	if cookie == nil || cookie.Value == "" {
		t.Fatal("no jwt cookie after login")
	}
	if !cookie.HttpOnly {
		t.Error("jwt cookie should be HttpOnly")
	}

	expectStatus(t, a.do("alice", "GET", "/chores/", nil), http.StatusOK, "authenticated list")

	rec = a.do("alice", "POST", "/logout", nil)
	expectStatus(t, rec, http.StatusSeeOther, "logout")
	for _, c := range rec.Result().Cookies() {
		if c.Name == "jwt" && c.Value != "" {
			t.Error("logout did not clear the cookie")
		}
	}
}

func TestOwnerCanViewOwnPages(t *testing.T) {
	a, s := newTestApp(t)
	for _, p := range []string{
		"/", "/chores/", "/chores/new", fmt.Sprintf("/chores/%d/", s.aliceChore), fmt.Sprintf("/chores/%d/edit", s.aliceChore),
		"/reminders/", "/reminders/new", fmt.Sprintf("/reminders/%d/", s.aliceReminder), fmt.Sprintf("/reminders/%d/edit", s.aliceReminder),
		"/tasks/", "/tasks/new", fmt.Sprintf("/tasks/%d/", s.aliceTask), fmt.Sprintf("/tasks/%d/edit", s.aliceTask),
	} {
		expectStatus(t, a.do("alice", "GET", p, nil), http.StatusOK, "GET "+p)
	}
}

func TestUsersCannotAccessEachOthersData(t *testing.T) {
	a, s := newTestApp(t)

	chore := fmt.Sprintf("/chores/%d/", s.aliceChore)
	reminder := fmt.Sprintf("/reminders/%d/", s.aliceReminder)
	task := fmt.Sprintf("/tasks/%d/", s.aliceTask)

	edit := url.Values{"name": {"hacked"}, "description": {"hacked"}, "time": {"1"}, "date": {"2030-01-01"}, "interval": {"Daily"}, "choreID": {fmt.Sprint(s.bobChore)}}
	del := url.Values{"action": {"delete"}}

	cases := []struct {
		method, path string
		form         url.Values
	}{
		{"GET", chore, nil}, {"POST", chore, del},
		{"GET", chore + "edit", nil}, {"POST", chore + "edit", edit},
		{"GET", reminder, nil}, {"POST", reminder, del},
		{"GET", reminder + "edit", nil}, {"POST", reminder + "edit", edit},
		{"GET", task, nil}, {"POST", task, del},
		{"GET", task + "edit", nil}, {"POST", task + "edit", edit},
	}
	for _, c := range cases {
		rec := a.do("bob", c.method, c.path, c.form)
		expectStatus(t, rec, http.StatusNotFound, "bob "+c.method+" "+c.path)
		if strings.Contains(rec.Body.String(), "alice chore") || strings.Contains(rec.Body.String(), "alice task") {
			t.Errorf("bob %s %s: leaked alice's data", c.method, c.path)
		}
	}

	// Nothing of alice's was changed or deleted.
	var gotChore models.Chore
	database.DB.First(&gotChore, s.aliceChore)
	if gotChore.Name != "alice chore" || gotChore.Time != 10 {
		t.Errorf("alice's chore was modified: %+v", gotChore)
	}
	var gotReminder models.ChoreReminder
	database.DB.First(&gotReminder, s.aliceReminder)
	if gotReminder.Interval != "Weekly" || gotReminder.ChoreID != uint64(s.aliceChore) {
		t.Errorf("alice's reminder was modified: %+v", gotReminder)
	}
	var gotTask models.Task
	database.DB.First(&gotTask, s.aliceTask)
	if gotTask.Name != "alice task" {
		t.Errorf("alice's task was modified: %+v", gotTask)
	}

	// Home-page actions on someone else's reminder.
	for _, action := range []string{"done", "delete"} {
		rec := a.do("bob", "POST", "/", url.Values{"reminderID": {fmt.Sprint(s.aliceReminder)}, "action": {action}})
		expectStatus(t, rec, http.StatusNotFound, "bob home "+action)
	}
	database.DB.First(&gotReminder, s.aliceReminder)
	if !gotReminder.Date.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("alice's reminder date was changed by bob: %v", gotReminder.Date)
	}

	// The lists only contain the caller's own rows.
	for _, p := range []string{"/chores/", "/tasks/", "/", "/reminders/"} {
		body := a.do("bob", "GET", p, nil).Body.String()
		if strings.Contains(body, "alice chore") || strings.Contains(body, "alice task") {
			t.Errorf("bob GET %s shows alice's data", p)
		}
	}
}

func TestReminderCannotReferenceAnotherUsersChore(t *testing.T) {
	a, s := newTestApp(t)

	before := count(t, &models.ChoreReminder{}, "user_id <> 0")
	rec := a.do("bob", "POST", "/reminders/new", url.Values{
		"date": {"2026-12-01"}, "choreID": {fmt.Sprint(s.aliceChore)}, "interval": {"Daily"},
	})
	expectStatus(t, rec, http.StatusBadRequest, "create reminder on alice's chore")
	if after := count(t, &models.ChoreReminder{}, "user_id <> 0"); after != before {
		t.Errorf("reminder was created (%d -> %d)", before, after)
	}

	rec = a.do("bob", "POST", fmt.Sprintf("/reminders/%d/edit", s.bobReminder), url.Values{
		"date": {"2026-12-01"}, "choreID": {fmt.Sprint(s.aliceChore)}, "interval": {"Daily"},
	})
	expectStatus(t, rec, http.StatusBadRequest, "edit reminder to alice's chore")
	var r models.ChoreReminder
	database.DB.First(&r, s.bobReminder)
	if r.ChoreID != uint64(s.bobChore) {
		t.Errorf("bob's reminder now points at chore %d", r.ChoreID)
	}

	// Pointing at your own chore is fine.
	rec = a.do("bob", "POST", "/reminders/new", url.Values{
		"date": {"2026-12-01"}, "choreID": {fmt.Sprint(s.bobChore)}, "interval": {"Daily"},
	})
	expectStatus(t, rec, http.StatusFound, "create reminder on own chore")
}

func TestInvalidFormsAreRejectedWithoutCreatingRows(t *testing.T) {
	a, s := newTestApp(t)

	chores := count(t, &models.Chore{}, "1 = 1")
	reminders := count(t, &models.ChoreReminder{}, "1 = 1")
	tasks := count(t, &models.Task{}, "1 = 1")

	expectStatus(t, a.do("alice", "POST", "/chores/new", url.Values{"name": {"x"}, "time": {"abc"}}), http.StatusBadRequest, "bad chore time")
	expectStatus(t, a.do("alice", "POST", "/reminders/new", url.Values{"date": {"bad"}, "choreID": {fmt.Sprint(s.aliceChore)}, "interval": {"Daily"}}), http.StatusBadRequest, "bad reminder date")
	expectStatus(t, a.do("alice", "POST", "/reminders/new", url.Values{"date": {"2026-12-01"}, "choreID": {fmt.Sprint(s.aliceChore)}, "interval": {"Hourly"}}), http.StatusBadRequest, "bad reminder interval")
	expectStatus(t, a.do("alice", "POST", "/tasks/new", url.Values{"name": {"x"}, "date": {"bad"}}), http.StatusBadRequest, "bad task date")

	if count(t, &models.Chore{}, "1 = 1") != chores || count(t, &models.ChoreReminder{}, "1 = 1") != reminders || count(t, &models.Task{}, "1 = 1") != tasks {
		t.Error("an invalid form created a row")
	}

	for _, p := range []string{"/chores/999/", "/chores/abc/edit", "/reminders/999/", "/reminders/999/edit", "/tasks/999/", "/tasks/abc/"} {
		rec := a.do("alice", "GET", p, nil)
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s: got %d, want 404 or 400", p, rec.Code)
		}
	}
}

func TestOwnerCanCreateEditAndDelete(t *testing.T) {
	a, s := newTestApp(t)

	rec := a.do("alice", "POST", "/chores/new", url.Values{"name": {"Mop"}, "description": {"floors"}, "time": {"20"}})
	expectStatus(t, rec, http.StatusFound, "create chore")
	if count(t, &models.Chore{}, "name = ? AND user_id <> 0", "Mop") != 1 {
		t.Error("chore not created")
	}

	rec = a.do("alice", "POST", fmt.Sprintf("/chores/%d/edit", s.aliceChore), url.Values{"name": {"Renamed"}, "description": {"x"}, "time": {"30"}})
	expectStatus(t, rec, http.StatusSeeOther, "edit chore")
	var c models.Chore
	database.DB.First(&c, s.aliceChore)
	if c.Name != "Renamed" || c.Time != 30 {
		t.Errorf("chore not updated: %+v", c)
	}

	rec = a.do("alice", "POST", fmt.Sprintf("/tasks/%d/", s.aliceTask), url.Values{"action": {"delete"}})
	expectStatus(t, rec, http.StatusFound, "delete task")
	if count(t, &models.Task{}, "id = ?", s.aliceTask) != 0 {
		t.Error("task not deleted")
	}

	rec = a.do("alice", "POST", fmt.Sprintf("/reminders/%d/", s.aliceReminder), url.Values{"action": {"delete"}})
	expectStatus(t, rec, http.StatusFound, "delete reminder")
	if count(t, &models.ChoreReminder{}, "id = ?", s.aliceReminder) != 0 {
		t.Error("reminder not deleted")
	}

	rec = a.do("alice", "POST", fmt.Sprintf("/chores/%d/", s.aliceChore), url.Values{"action": {"delete"}})
	expectStatus(t, rec, http.StatusFound, "delete chore")
	if count(t, &models.Chore{}, "id = ?", s.aliceChore) != 0 {
		t.Error("chore not deleted")
	}
}

func TestMarkingRemindersDone(t *testing.T) {
	a, s := newTestApp(t)

	// A recurring reminder moves into the future.
	rec := a.do("alice", "POST", "/", url.Values{"reminderID": {fmt.Sprint(s.aliceReminder)}, "action": {"done"}})
	expectStatus(t, rec, http.StatusFound, "done weekly")
	var r models.ChoreReminder
	database.DB.First(&r, s.aliceReminder)
	if r.Date.Before(time.Now().AddDate(0, 0, 6)) {
		t.Errorf("weekly reminder not rescheduled: %v", r.Date)
	}

	// A one-off reminder is removed.
	database.DB.Model(&models.ChoreReminder{}).Where("id = ?", s.bobReminder).Update("interval", "Once")
	rec = a.do("bob", "POST", "/", url.Values{"reminderID": {fmt.Sprint(s.bobReminder)}, "action": {"done"}})
	expectStatus(t, rec, http.StatusFound, "done once")
	if count(t, &models.ChoreReminder{}, "id = ?", s.bobReminder) != 0 {
		t.Error("one-off reminder not deleted")
	}

	expectStatus(t, a.do("alice", "POST", "/", url.Values{"action": {"done"}}), http.StatusBadRequest, "missing reminderID")
}

func TestTokensExpireAndDieWithTheirUser(t *testing.T) {
	a, s := newTestApp(t)

	ck := a.login("alice")
	tok, err := jwtauth.VerifyToken(jwtauth.New("HS256", []byte(testSecret), nil), ck.Value)
	if err != nil {
		t.Fatal(err)
	}
	if tok.Expiration().IsZero() {
		t.Fatal("login token has no exp claim")
	}
	if d := time.Until(tok.Expiration()); d < 23*time.Hour || d > 25*time.Hour {
		t.Errorf("token expires in %v, want about 24h", d)
	}

	expectStatus(t, a.do("alice", "GET", "/chores/", nil), http.StatusOK, "before user deletion")

	// Deleting the user invalidates tokens already issued to them.
	if err := database.DB.Delete(&models.User{}, "username = ?", "alice").Error; err != nil {
		t.Fatal(err)
	}
	rec := a.do("alice", "GET", "/chores/", nil)
	expectStatus(t, rec, http.StatusFound, "token of deleted user")
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("redirect to %q, want /login", loc)
	}
	rec = a.do("alice", "POST", fmt.Sprintf("/chores/%d/", s.aliceChore), url.Values{"action": {"delete"}})
	expectStatus(t, rec, http.StatusFound, "deleted user's POST")
	if count(t, &models.Chore{}, "id = ?", s.aliceChore) != 1 {
		t.Error("deleted user's token was still able to delete data")
	}

	// Other users are unaffected.
	expectStatus(t, a.do("bob", "GET", "/chores/", nil), http.StatusOK, "other user")
}

func TestEditsValidateInput(t *testing.T) {
	a, s := newTestApp(t)

	chore := fmt.Sprintf("/chores/%d/edit", s.aliceChore)
	reminder := fmt.Sprintf("/reminders/%d/edit", s.aliceReminder)
	task := fmt.Sprintf("/tasks/%d/edit", s.aliceTask)
	ownChore := fmt.Sprint(s.aliceChore)

	bad := []struct {
		what, path string
		form       url.Values
	}{
		{"chore empty name", chore, url.Values{"name": {""}, "time": {"5"}}},
		{"chore blank name", chore, url.Values{"name": {"   "}, "time": {"5"}}},
		{"chore negative time", chore, url.Values{"name": {"x"}, "time": {"-5"}}},
		{"chore non-numeric time", chore, url.Values{"name": {"x"}, "time": {"abc"}}},
		{"reminder bogus interval", reminder, url.Values{"choreID": {ownChore}, "date": {"2026-12-01"}, "interval": {"Hourly"}}},
		{"reminder bad date", reminder, url.Values{"choreID": {ownChore}, "date": {"garbage"}, "interval": {"Daily"}}},
		{"reminder missing chore", reminder, url.Values{"date": {"2026-12-01"}, "interval": {"Daily"}}},
		{"task empty name", task, url.Values{"name": {""}, "date": {"2026-12-01"}}},
		{"task bad date", task, url.Values{"name": {"x"}, "date": {"garbage"}}},
	}
	for _, c := range bad {
		expectStatus(t, a.do("alice", "POST", c.path, c.form), http.StatusBadRequest, c.what)
	}

	// None of the rejected edits changed anything.
	var gc models.Chore
	database.DB.First(&gc, s.aliceChore)
	if gc.Name != "alice chore" || gc.Time != 10 {
		t.Errorf("chore changed: %+v", gc)
	}
	var gr models.ChoreReminder
	database.DB.First(&gr, s.aliceReminder)
	if gr.Interval != "Weekly" || !gr.Date.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("reminder changed: %+v", gr)
	}
	var gt models.Task
	database.DB.First(&gt, s.aliceTask)
	if gt.Name != "alice task" {
		t.Errorf("task changed: %+v", gt)
	}

	// Creating with an empty name is rejected too.
	n := count(t, &models.Chore{}, "1 = 1")
	expectStatus(t, a.do("alice", "POST", "/chores/new", url.Values{"name": {""}, "time": {"5"}}), http.StatusBadRequest, "create chore empty name")
	expectStatus(t, a.do("alice", "POST", "/tasks/new", url.Values{"name": {""}, "date": {"2026-12-01"}}), http.StatusBadRequest, "create task empty name")
	if count(t, &models.Chore{}, "1 = 1") != n {
		t.Error("chore with empty name was created")
	}

	// Valid edits still work and redirect to the detail page.
	rec := a.do("alice", "POST", reminder, url.Values{"choreID": {ownChore}, "date": {"2026-12-01"}, "interval": {"Monthly"}})
	expectStatus(t, rec, http.StatusSeeOther, "valid reminder edit")
	if loc := rec.Header().Get("Location"); loc != fmt.Sprintf("/reminders/%d/", s.aliceReminder) {
		t.Errorf("reminder edit redirected to %q", loc)
	}
	database.DB.First(&gr, s.aliceReminder)
	if gr.Interval != "Monthly" || !gr.Date.Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("reminder not updated: %+v", gr)
	}
	rec = a.do("alice", "POST", chore, url.Values{"name": {"New name"}, "description": {"d"}, "time": {"0"}})
	expectStatus(t, rec, http.StatusSeeOther, "valid chore edit")
	if loc := rec.Header().Get("Location"); loc != fmt.Sprintf("/chores/%d/", s.aliceChore) {
		t.Errorf("chore edit redirected to %q", loc)
	}
}
