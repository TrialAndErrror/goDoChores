package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"goDoChores/database"
	"goDoChores/models"
)

func addUser(t *testing.T, name string) models.User {
	t.Helper()
	u := models.User{Username: name, Email: name + "@example.com"}
	if err := u.SetPassword(name + "-pw"); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	return u
}

func userID(t *testing.T, name string) uint {
	t.Helper()
	var u models.User
	if err := database.DB.Where("username = ?", name).First(&u).Error; err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func form(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i < len(kv); i += 2 {
		v.Add(kv[i], kv[i+1])
	}
	return v
}

func TestGroupInviteLifecycle(t *testing.T) {
	app, _ := newTestApp(t)

	expectStatus(t, app.do("alice", "POST", "/groups/new", form("name", "  Home  ")), http.StatusSeeOther, "create group")
	var g models.Group
	if err := database.DB.First(&g).Error; err != nil || g.Name != "Home" {
		t.Fatalf("group = %+v, err %v", g, err)
	}
	expectStatus(t, app.do("alice", "POST", "/groups/new", form("name", "Again")), http.StatusConflict, "second group")
	expectStatus(t, app.do("bob", "POST", "/groups/rename", form("name", "Hacked")), http.StatusForbidden, "non-owner rename")
	expectStatus(t, app.do("alice", "POST", "/groups/rename", form("name", "Our Home")), http.StatusSeeOther, "rename")
	if body := app.do("alice", "GET", "/groups/", nil).Body.String(); !strings.Contains(body, "Our Home") {
		t.Error("group page does not show the new name")
	}

	expectStatus(t, app.do("alice", "POST", "/groups/invite", form("email", "nonsense")), http.StatusBadRequest, "bad email")
	expectStatus(t, app.do("bob", "POST", "/groups/invite", form("email", "carol@example.com")), http.StatusForbidden, "non-owner invite")
	// Mixed case and repeats collapse into one pending invite.
	expectStatus(t, app.do("alice", "POST", "/groups/invite", form("email", "Bob@Example.com")), http.StatusSeeOther, "invite")
	expectStatus(t, app.do("alice", "POST", "/groups/invite", form("email", "bob@example.com")), http.StatusSeeOther, "re-invite")
	if n := count(t, &models.GroupInvite{}, "1 = 1"); n != 1 {
		t.Fatalf("pending invites = %d, want 1", n)
	}

	var inv models.GroupInvite
	database.DB.First(&inv)
	answer := "/groups/invites/" + itoa(inv.ID) + "/accept"
	expectStatus(t, app.do("alice", "POST", answer, form()), http.StatusNotFound, "invite is private to its recipient")
	expectStatus(t, app.do("bob", "POST", answer, form()), http.StatusSeeOther, "accept")
	if n := count(t, &models.GroupMember{}, "group_id = ?", g.ID); n != 2 {
		t.Errorf("members = %d, want 2", n)
	}
	if n := count(t, &models.GroupInvite{}, "1 = 1"); n != 0 {
		t.Errorf("invite should be consumed, %d left", n)
	}
	expectStatus(t, app.do("alice", "POST", "/groups/invite", form("email", "bob@example.com")), http.StatusConflict, "invite a member")

	expectStatus(t, app.do("alice", "POST", "/groups/leave", form()), http.StatusConflict, "owner cannot leave")
	expectStatus(t, app.do("bob", "POST", "/groups/leave", form()), http.StatusSeeOther, "leave")
	if n := count(t, &models.GroupMember{}, "group_id = ?", g.ID); n != 1 {
		t.Errorf("members = %d, want 1", n)
	}
}

func TestRejectingAndConflictingInvites(t *testing.T) {
	app, _ := newTestApp(t)
	carol := addUser(t, "carol")

	app.do("alice", "POST", "/groups/new", form("name", "A"))
	app.do("bob", "POST", "/groups/new", form("name", "B"))
	app.do("alice", "POST", "/groups/invite", form("email", carol.Email))
	app.do("bob", "POST", "/groups/invite", form("email", carol.Email))

	var invs []models.GroupInvite
	database.DB.Order("id").Find(&invs)
	if len(invs) != 2 {
		t.Fatalf("invites = %d, want 2", len(invs))
	}
	expectStatus(t, app.do("carol", "POST", "/groups/invites/"+itoa(invs[0].ID)+"/reject", form()), http.StatusSeeOther, "reject")
	if n := count(t, &models.GroupMember{}, "user_id = ?", carol.ID); n != 0 {
		t.Error("rejecting must not join the group")
	}
	expectStatus(t, app.do("carol", "POST", "/groups/invites/"+itoa(invs[1].ID)+"/accept", form()), http.StatusSeeOther, "accept second")
	// Carol is now in B; another invite to A cannot be accepted while she is.
	app.do("alice", "POST", "/groups/invite", form("email", carol.Email))
	var latest models.GroupInvite
	database.DB.Order("id desc").First(&latest)
	expectStatus(t, app.do("carol", "POST", "/groups/invites/"+itoa(latest.ID)+"/accept", form()), http.StatusConflict, "accept while in a group")
}

func TestDeleteGroup(t *testing.T) {
	app, _ := newTestApp(t)
	app.do("alice", "POST", "/groups/new", form("name", "A"))
	app.do("alice", "POST", "/groups/invite", form("email", "bob@example.com"))
	expectStatus(t, app.do("bob", "POST", "/groups/delete", form()), http.StatusForbidden, "non-owner delete")
	expectStatus(t, app.do("alice", "POST", "/groups/delete", form()), http.StatusSeeOther, "delete")
	for _, m := range []any{&models.Group{}, &models.GroupMember{}, &models.GroupInvite{}} {
		if n := count(t, m, "1 = 1"); n != 0 {
			t.Errorf("%T rows left after delete: %d", m, n)
		}
	}
}

// groupWithMembers makes alice's group and adds bob to it.
func groupWithMembers(t *testing.T, app *testApp) models.Group {
	t.Helper()
	app.do("alice", "POST", "/groups/new", form("name", "G"))
	app.do("alice", "POST", "/groups/invite", form("email", "bob@example.com"))
	var inv models.GroupInvite
	database.DB.First(&inv)
	app.do("bob", "POST", "/groups/invites/"+itoa(inv.ID)+"/accept", form())
	var g models.Group
	database.DB.First(&g)
	return g
}

func TestNormalizeTag(t *testing.T) {
	if got, err := models.NormalizeTag("  Kitchen "); err != nil || got != "kitchen" {
		t.Errorf("got %q, %v", got, err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("x", 31)} {
		if _, err := models.NormalizeTag(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestTagsAreSharedWithinAGroup(t *testing.T) {
	app, s := newTestApp(t)

	// Without a group the dropdown still works, with personal tags; creating the
	// group below folds this one into it.
	expectStatus(t, app.do("alice", "POST", "/groups/tags/new", form("name", "kitchen")), http.StatusOK, "tag without a group")
	if b := app.do("alice", "GET", "/chores/new", nil).Body.String(); !strings.Contains(b, "Add New") {
		t.Error("tag dropdown should show without a group")
	}

	g := groupWithMembers(t, app)

	// Creating is idempotent and case-insensitive, and answers with JSON for the dropdown.
	rec := app.do("alice", "POST", "/groups/tags/new", form("name", " Kitchen "))
	expectStatus(t, rec, http.StatusOK, "create tag")
	if !strings.Contains(rec.Body.String(), `"name":"kitchen"`) {
		t.Errorf("unexpected body %q", rec.Body.String())
	}
	app.do("bob", "POST", "/groups/tags/new", form("name", "KITCHEN"))
	expectStatus(t, app.do("bob", "POST", "/groups/tags/new", form("name", "")), http.StatusBadRequest, "empty tag")
	if n := count(t, &models.Tag{}, "group_id = ?", g.ID); n != 1 {
		t.Fatalf("tags = %d, want 1 shared tag", n)
	}
	var tag models.Tag
	database.DB.First(&tag)

	// Both members' chore forms offer it.
	for _, u := range []string{"alice", "bob"} {
		if b := app.do(u, "GET", "/chores/new", nil).Body.String(); !strings.Contains(b, `value="`+itoa(tag.ID)+`"`) || !strings.Contains(b, "Add New") {
			t.Errorf("%s's chore form should list the shared tag", u)
		}
	}

	// Tag a chore, and see it in the edit form (checked) and detail page.
	path := "/chores/" + itoa(s.aliceChore) + "/edit"
	expectStatus(t, app.do("alice", "POST", path, form("name", "Dishes", "time", "5", "tag_ids", itoa(tag.ID))), http.StatusSeeOther, "tag a chore")
	if b := app.do("alice", "GET", path, nil).Body.String(); !strings.Contains(b, `id="tag-`+itoa(tag.ID)+`" checked`) {
		t.Error("edit form should have the tag checked")
	}
	if b := app.do("alice", "GET", "/chores/"+itoa(s.aliceChore)+"/", nil).Body.String(); !strings.Contains(b, "kitchen") {
		t.Error("detail page should show the tag")
	}

	// Another group's tag (or a made-up id) can't be put on a chore.
	carol := addUser(t, "carol")
	other := models.Group{Name: "O", OwnerID: carol.ID}
	database.DB.Create(&other)
	database.DB.Create(&models.GroupMember{GroupID: other.ID, UserID: carol.ID})
	foreign := models.Tag{GroupID: &other.ID, Name: "secret"}
	database.DB.Create(&foreign)
	expectStatus(t, app.do("alice", "POST", path, form("name", "Dishes", "time", "5", "tag_ids", itoa(foreign.ID))), http.StatusBadRequest, "foreign tag")
	expectStatus(t, app.do("alice", "POST", path, form("name", "Dishes", "time", "5", "tag_ids", "abc")), http.StatusBadRequest, "bad tag id")

	// Deleting the chore drops its tag links but keeps the shared tag.
	expectStatus(t, app.do("alice", "POST", "/chores/"+itoa(s.aliceChore)+"/", form("action", "delete")), http.StatusFound, "delete chore")
	if n := count(t, &models.ChoreTag{}, "chore_id = ?", s.aliceChore); n != 0 {
		t.Errorf("tag links left after chore delete: %d", n)
	}
	if n := count(t, &models.Tag{}, "id = ?", tag.ID); n != 1 {
		t.Error("shared tag should survive")
	}
}

func TestLeavingAGroupKeepsYourTagsAsPersonalOnes(t *testing.T) {
	app, s := newTestApp(t)
	g := groupWithMembers(t, app)
	tag := models.Tag{GroupID: &g.ID, Name: "garage"}
	database.DB.Create(&tag)
	database.DB.Create(&models.ChoreTag{ChoreID: s.bobChore, TagID: tag.ID})
	database.DB.Create(&models.ChoreTag{ChoreID: s.aliceChore, TagID: tag.ID})
	app.do("bob", "POST", "/groups/tags", form("all", itoa(tag.ID)))

	expectStatus(t, app.do("bob", "POST", "/groups/leave", form()), http.StatusSeeOther, "leave")
	if n := count(t, &models.HiddenTag{}, "1 = 1"); n != 0 {
		t.Error("leaving should clear your Tags to Show choices")
	}
	// Bob's chore now carries his own "garage", not the group's.
	var bobTag models.Tag
	if err := database.DB.Joins("join chore_tags on chore_tags.tag_id = tags.id").
		Where("chore_tags.chore_id = ?", s.bobChore).First(&bobTag).Error; err != nil {
		t.Fatalf("bob's chore lost its tag: %v", err)
	}
	if bobTag.Name != "garage" || bobTag.GroupID != nil || bobTag.OwnerID == nil || *bobTag.OwnerID != userID(t, "bob") {
		t.Errorf("want a personal garage tag owned by bob, got %+v", bobTag)
	}
	if n := count(t, &models.ChoreTag{}, "chore_id = ? AND tag_id = ?", s.aliceChore, tag.ID); n != 1 {
		t.Error("the remaining member's tags must be untouched")
	}
	// The personal tag is what bob's chore form offers now.
	if b := app.do("bob", "GET", "/chores/new", nil).Body.String(); !strings.Contains(b, `value="`+itoa(bobTag.ID)+`"`) || strings.Contains(b, `value="`+itoa(tag.ID)+`"`) {
		t.Error("bob's tag picker should list his personal tag and not the group's")
	}

	// Deleting the group gives its remaining members personal copies too.
	expectStatus(t, app.do("alice", "POST", "/groups/delete", form()), http.StatusSeeOther, "delete group")
	if n := count(t, &models.Tag{}, "group_id IS NOT NULL"); n != 0 {
		t.Errorf("group tags left after group delete: %d", n)
	}
	if n := count(t, &models.Tag{}, "owner_id = ? AND name = ?", userID(t, "alice"), "garage"); n != 1 {
		t.Error("alice should keep a personal garage tag after the group is deleted")
	}
	if n := count(t, &models.ChoreTag{}, "chore_id = ?", s.aliceChore); n != 1 {
		t.Error("alice's chore should stay tagged")
	}
	if n := count(t, &models.HiddenTag{}, "1 = 1"); n != 0 {
		t.Errorf("hidden tag rows left: %d", n)
	}
}

func TestPersonalTagsMergeIntoTheGroupOnJoin(t *testing.T) {
	app, s := newTestApp(t)

	// Without a group, tags are personal.
	resp := app.do("bob", "POST", "/groups/tags/new", form("name", "Garage"))
	expectStatus(t, resp, http.StatusOK, "solo tag")
	var made struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &made); err != nil || made.Name != "garage" {
		t.Fatalf("bad tag response %q: %v", resp.Body.String(), err)
	}
	database.DB.Create(&models.ChoreTag{ChoreID: s.bobChore, TagID: made.ID})
	if n := count(t, &models.Tag{}, "owner_id = ? AND group_id IS NULL", userID(t, "bob")); n != 1 {
		t.Fatal("solo tag should be personal")
	}

	// Alice's group already has "garage" and she tags her chore with it.
	app.do("alice", "POST", "/groups/new", form("name", "Home"))
	var g models.Group
	database.DB.Where("owner_id = ?", userID(t, "alice")).First(&g)
	shared := models.Tag{GroupID: &g.ID, Name: "garage"}
	database.DB.Create(&shared)
	database.DB.Create(&models.ChoreTag{ChoreID: s.aliceChore, TagID: shared.ID})

	app.do("alice", "POST", "/groups/invite", form("email", "bob@example.com"))
	var inv models.GroupInvite
	database.DB.First(&inv)
	expectStatus(t, app.do("bob", "POST", "/groups/invites/"+itoa(inv.ID)+"/accept", form()), http.StatusSeeOther, "accept")

	if n := count(t, &models.Tag{}, "owner_id IS NOT NULL"); n != 0 {
		t.Errorf("personal tags left after joining: %d", n)
	}
	if n := count(t, &models.Tag{}, "name = ?", "garage"); n != 1 {
		t.Errorf("want one garage tag after the merge, got %d", n)
	}
	if n := count(t, &models.ChoreTag{}, "chore_id = ? AND tag_id = ?", s.bobChore, shared.ID); n != 1 {
		t.Error("bob's chore should now carry the group's garage tag")
	}
}

func TestHomeShowsGroupChoresAndFiltersByTag(t *testing.T) {
	app, s := newTestApp(t)
	db := database.DB
	carol := addUser(t, "carol")
	cc := models.Chore{Name: "carol chore", Time: 1, UserID: carol.ID}
	db.Create(&cc)
	db.Create(&models.ChoreReminder{ChoreID: uint64(cc.ID), Date: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Interval: "Weekly", UserID: carol.ID})

	home := func() string { return app.do("alice", "GET", "/", nil).Body.String() }
	if strings.Contains(home(), "bob chore") {
		t.Fatal("bob's chore visible before any group exists")
	}

	g := groupWithMembers(t, app)
	garage := models.Tag{GroupID: &g.ID, Name: "garage"}
	db.Create(&garage)
	db.Create(&models.ChoreTag{ChoreID: s.bobChore, TagID: garage.ID})

	b := home()
	if !strings.Contains(b, "bob chore") || !strings.Contains(b, "alice chore") {
		t.Fatal("group member's chore should show alongside your own")
	}
	if strings.Contains(b, "carol chore") {
		t.Error("non-member's chore must not show")
	}
	if strings.Contains(b, "Tags to Show") {
		t.Error("the home page should have no tag filter")
	}
	if gp := app.do("alice", "GET", "/groups/", nil).Body.String(); !strings.Contains(gp, "Tags to Show") || !strings.Contains(gp, `value="`+itoa(garage.ID)+`"`) {
		t.Error("group page should list the group's tags under Tags to Show")
	}

	// Hide "garage": bob's chore goes for alice only.
	expectStatus(t, app.do("alice", "POST", "/groups/tags", form("all", itoa(garage.ID))), http.StatusSeeOther, "save tags")
	b = home()
	if strings.Contains(b, "bob chore") || !strings.Contains(b, "alice chore") {
		t.Error("tag filter should hide only the tagged chore")
	}
	if !strings.Contains(app.do("bob", "GET", "/", nil).Body.String(), "bob chore") {
		t.Error("alice's setting leaked to bob")
	}

	// Re-checking shows it again.
	app.do("alice", "POST", "/groups/tags", form("all", itoa(garage.ID), "show", itoa(garage.ID)))
	if !strings.Contains(home(), "bob chore") {
		t.Error("chore should return once the tag is shown again")
	}

	// A tag from another group can't be saved into the setting.
	foreignGroup := uint(999)
	foreign := models.Tag{GroupID: &foreignGroup, Name: "x"}
	db.Create(&foreign)
	app.do("alice", "POST", "/groups/tags", form("all", itoa(foreign.ID)))
	if n := count(t, &models.HiddenTag{}, "tag_id = ?", foreign.ID); n != 0 {
		t.Error("foreign tag was saved")
	}

	if b := home(); !strings.Contains(b, `data-owner="bob"`) {
		t.Error("bob's reminder should be clickable and labelled with its owner")
	}

	// Members can complete each other's reminders, but not delete them.
	expectStatus(t, app.do("alice", "POST", "/", form("reminderID", itoa(s.bobReminder), "action", "delete")), http.StatusForbidden, "delete member's reminder")
	var before, after models.ChoreReminder
	db.First(&before, s.bobReminder)
	expectStatus(t, app.do("alice", "POST", "/", form("reminderID", itoa(s.bobReminder), "action", "done")), http.StatusFound, "complete member's reminder")
	db.First(&after, s.bobReminder)
	if !after.Date.After(before.Date) {
		t.Errorf("reminder date did not advance: %v -> %v", before.Date, after.Date)
	}
	var cr models.ChoreReminder
	db.Where("user_id = ?", carol.ID).First(&cr)
	expectStatus(t, app.do("alice", "POST", "/", form("reminderID", itoa(cr.ID), "action", "done")), http.StatusNotFound, "complete outsider's reminder")
}

func itoa(n uint) string { return strconv.FormatUint(uint64(n), 10) }
