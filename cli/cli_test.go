package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goDoChores/database"
	"goDoChores/models"

	"github.com/spf13/cobra"
)

// run executes the CLI the way main does, against a throwaway database.
func run(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "app", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(Commands()...)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func freshDB(t *testing.T) {
	t.Helper()
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "fresh.db"))
}

func mustRun(t *testing.T, stdin string, args ...string) string {
	t.Helper()
	out, err := run(t, stdin, args...)
	if err != nil {
		t.Fatalf("%v failed: %v\n%s", args, err, out)
	}
	return out
}

func findUser(t *testing.T, username string) (models.User, bool) {
	t.Helper()
	var u models.User
	err := database.DB.Where("username = ?", username).First(&u).Error
	return u, err == nil
}

func TestCreateUserWorksOnAFreshDatabase(t *testing.T) {
	freshDB(t)

	out := mustRun(t, "", "create-user", "-u", "alice", "-e", "alice@example.com", "-p", "pw-alice")
	if !strings.Contains(out, "User created: alice") {
		t.Errorf("unexpected output: %q", out)
	}
	u, ok := findUser(t, "alice")
	if !ok || u.Email != "alice@example.com" || !u.CheckPassword("pw-alice") {
		t.Errorf("user not stored correctly: %+v (found %v)", u, ok)
	}
	if u.PasswordHash == "pw-alice" {
		t.Error("password stored in plain text")
	}
}

func TestCreateUserPromptsForMissingValues(t *testing.T) {
	freshDB(t)

	mustRun(t, "bob\nbob@example.com\nbob-secret\n", "create-user")
	u, ok := findUser(t, "bob")
	if !ok || u.Email != "bob@example.com" || !u.CheckPassword("bob-secret") {
		t.Errorf("prompted user wrong: %+v (found %v)", u, ok)
	}

	// Flags and prompts mix: only the password is asked for.
	mustRun(t, "carol-secret\n", "create-user", "-u", "carol", "-e", "carol@example.com")
	if c, ok := findUser(t, "carol"); !ok || !c.CheckPassword("carol-secret") {
		t.Errorf("mixed flags/prompt user wrong: %+v (found %v)", c, ok)
	}

	// Values may contain spaces (the old Scanln-based prompt could not).
	mustRun(t, "dave smith\ndave@example.com\nhas spaces in it\n", "create-user")
	if d, ok := findUser(t, "dave smith"); !ok || !d.CheckPassword("has spaces in it") {
		t.Errorf("spaces not preserved: %+v (found %v)", d, ok)
	}
}

func TestCreateUserValidation(t *testing.T) {
	freshDB(t)

	cases := map[string][]string{
		"empty username":   {"create-user", "-u", "  ", "-e", "a@b.c", "-p", "x"},
		"empty email":      {"create-user", "-u", "a", "-e", "", "-p", "x"},
		"empty password":   {"create-user", "-u", "a", "-e", "a@b.c", "-p", ""},
		"no input at all":  {"create-user"},
		"input runs out":   {"create-user", "-u", "a", "-e", "a@b.c"},
		"unexpected extra": {"create-user", "stray-arg"},
	}
	for name, args := range cases {
		if _, err := run(t, "", args...); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if users, _ := ListUsers(database.DB); len(users) != 0 {
		t.Errorf("a rejected create left %d users behind", len(users))
	}
}

func TestCreateUserUpdatesAnExistingUser(t *testing.T) {
	freshDB(t)
	mustRun(t, "", "create-user", "-u", "alice", "-e", "old@example.com", "-p", "old-pw")

	out := mustRun(t, "", "create-user", "-u", "alice", "-e", "new@example.com", "-p", "new-pw")
	if !strings.Contains(out, "already existed") {
		t.Errorf("should say the user was updated: %q", out)
	}
	u, _ := findUser(t, "alice")
	if u.Email != "new@example.com" || !u.CheckPassword("new-pw") || u.CheckPassword("old-pw") {
		t.Errorf("user not updated: %+v", u)
	}
	if users, _ := ListUsers(database.DB); len(users) != 1 {
		t.Errorf("update created a second user: %d users", len(users))
	}
}

func TestCreateUserRejectsDuplicateEmail(t *testing.T) {
	freshDB(t)
	mustRun(t, "", "create-user", "-u", "alice", "-e", "shared@example.com", "-p", "a")

	if _, err := run(t, "", "create-user", "-u", "bob", "-e", "shared@example.com", "-p", "b"); err == nil {
		t.Fatal("expected an error for a duplicate email")
	}
	if _, ok := findUser(t, "bob"); ok {
		t.Error("bob was created despite the duplicate email")
	}
}

func TestListUsers(t *testing.T) {
	freshDB(t)

	out := mustRun(t, "", "list-users")
	if !strings.Contains(out, "No users.") {
		t.Errorf("empty list output: %q", out)
	}

	mustRun(t, "", "create-user", "-u", "alice", "-e", "alice@example.com", "-p", "a")
	mustRun(t, "", "create-user", "-u", "bob", "-e", "bob@example.com", "-p", "b")
	alice, _ := findUser(t, "alice")
	c := models.Chore{Name: "c", UserID: alice.ID}
	database.DB.Create(&c)
	database.DB.Create(&models.ChoreReminder{ChoreID: uint64(c.ID), Date: time.Now(), Interval: "Daily", UserID: alice.ID})
	database.DB.Create(&models.ChoreReminder{ChoreID: uint64(c.ID), Date: time.Now(), Interval: "Weekly", UserID: alice.ID})
	database.DB.Create(&models.Task{Name: "t", Date: time.Now(), UserID: alice.ID})

	out = mustRun(t, "", "list-users")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("want header + 2 users, got:\n%s", out)
	}
	for _, col := range []string{"ID", "USERNAME", "EMAIL", "CHORES", "REMINDERS", "TASKS", "CREATED"} {
		if !strings.Contains(lines[0], col) {
			t.Errorf("header missing %s: %q", col, lines[0])
		}
	}
	aliceFields := strings.Fields(lines[1])
	if aliceFields[1] != "alice" || aliceFields[2] != "alice@example.com" || aliceFields[3] != "1" || aliceFields[4] != "2" || aliceFields[5] != "1" {
		t.Errorf("alice row wrong: %q", lines[1])
	}
	bobFields := strings.Fields(lines[2])
	if bobFields[1] != "bob" || bobFields[3] != "0" || bobFields[4] != "0" || bobFields[5] != "0" {
		t.Errorf("bob row wrong: %q", lines[2])
	}
	if strings.Contains(out, "$2a$") {
		t.Error("list-users leaked a password hash")
	}
}

// seedData gives a user one chore, one reminder and one task.
func seedData(t *testing.T, username string) {
	t.Helper()
	u, ok := findUser(t, username)
	if !ok {
		t.Fatalf("no user %s", username)
	}
	c := models.Chore{Name: username + " chore", UserID: u.ID}
	database.DB.Create(&c)
	database.DB.Create(&models.ChoreReminder{ChoreID: uint64(c.ID), Date: time.Now(), Interval: "Daily", UserID: u.ID})
	database.DB.Create(&models.Task{Name: username + " task", Date: time.Now(), UserID: u.ID})
}

func rows(t *testing.T, model any, where string, args ...any) int64 {
	t.Helper()
	var n int64
	// Unscoped so soft-deleted leftovers are counted too.
	if err := database.DB.Unscoped().Model(model).Where(where, args...).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestDeleteUserRemovesTheUserAndTheirData(t *testing.T) {
	freshDB(t)
	mustRun(t, "", "create-user", "-u", "alice", "-e", "alice@example.com", "-p", "a")
	mustRun(t, "", "create-user", "-u", "bob", "-e", "bob@example.com", "-p", "b")
	seedData(t, "alice")
	seedData(t, "bob")
	alice, _ := findUser(t, "alice")

	out := mustRun(t, "", "delete-user", "alice", "--yes")
	if !strings.Contains(out, "Deleted user alice") || !strings.Contains(out, "1 chores, 1 reminders and 1 tasks") {
		t.Errorf("unexpected output: %q", out)
	}

	if rows(t, &models.User{}, "id = ?", alice.ID) != 0 {
		t.Error("user row still exists")
	}
	for name, model := range map[string]any{"chores": &models.Chore{}, "reminders": &models.ChoreReminder{}, "tasks": &models.Task{}} {
		if n := rows(t, model, "user_id = ?", alice.ID); n != 0 {
			t.Errorf("%d %s left behind (including soft-deleted)", n, name)
		}
	}

	// Bob is untouched.
	bob, ok := findUser(t, "bob")
	if !ok {
		t.Fatal("bob was deleted")
	}
	if rows(t, &models.Chore{}, "user_id = ?", bob.ID) != 1 || rows(t, &models.ChoreReminder{}, "user_id = ?", bob.ID) != 1 || rows(t, &models.Task{}, "user_id = ?", bob.ID) != 1 {
		t.Error("bob's data was affected")
	}

	// The username and email are free to use again.
	mustRun(t, "", "create-user", "-u", "alice", "-e", "alice@example.com", "-p", "again")
	if a, ok := findUser(t, "alice"); !ok || !a.CheckPassword("again") {
		t.Error("could not recreate the deleted user")
	}
}

func TestDeleteUserConfirmation(t *testing.T) {
	freshDB(t)
	mustRun(t, "", "create-user", "-u", "alice", "-e", "alice@example.com", "-p", "a")
	seedData(t, "alice")

	// The prompt shows what will be lost.
	out, err := run(t, "nope\n", "delete-user", "alice")
	if err == nil {
		t.Fatal("a wrong confirmation should abort")
	}
	if !strings.Contains(out, "alice@example.com") || !strings.Contains(out, "1 chores, 1 reminders and 1 tasks") {
		t.Errorf("prompt should describe what will be deleted: %q", out)
	}

	// Empty answers and closed stdin abort too.
	for name, stdin := range map[string]string{"empty line": "\n", "closed stdin": "", "different case": "ALICE\n"} {
		if _, err := run(t, stdin, "delete-user", "alice"); err == nil {
			t.Errorf("%s: should abort", name)
		}
	}
	if _, ok := findUser(t, "alice"); !ok || rows(t, &models.Task{}, "user_id <> 0") != 1 {
		t.Fatal("an aborted delete removed something")
	}

	mustRun(t, "alice\n", "delete-user", "alice")
	if _, ok := findUser(t, "alice"); ok {
		t.Error("typing the username should confirm the delete")
	}
}

func TestDeleteUserErrors(t *testing.T) {
	freshDB(t)
	mustRun(t, "", "create-user", "-u", "alice", "-e", "alice@example.com", "-p", "a")

	if _, err := run(t, "", "delete-user", "ghost", "--yes"); err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Errorf("unknown user: got %v", err)
	}
	if _, err := run(t, "", "delete-user"); err == nil {
		t.Error("missing username should be an error")
	}
	if _, err := run(t, "", "delete-user", "a", "b"); err == nil {
		t.Error("two usernames should be an error")
	}
	if _, ok := findUser(t, "alice"); !ok {
		t.Error("alice should be untouched")
	}
}

func TestDeleteUserCleansUpGroupsAndTags(t *testing.T) {
	freshDB(t)
	mustRun(t, "", "create-user", "-u", "alice", "-e", "alice@example.com", "-p", "a")
	mustRun(t, "", "create-user", "-u", "bob", "-e", "bob@example.com", "-p", "b")
	seedData(t, "alice")
	alice, _ := findUser(t, "alice")
	bob, _ := findUser(t, "bob")

	db := database.DB
	var chore models.Chore
	db.Where("user_id = ?", alice.ID).First(&chore)
	g := models.Group{Name: "G", OwnerID: alice.ID}
	db.Create(&g)
	tag := models.Tag{GroupID: &g.ID, Name: "kitchen"}
	db.Create(&tag)
	db.Create(&models.ChoreTag{ChoreID: chore.ID, TagID: tag.ID})
	db.Create(&models.HiddenTag{UserID: bob.ID, TagID: tag.ID})
	db.Create(&models.GroupMember{GroupID: g.ID, UserID: alice.ID})
	db.Create(&models.GroupMember{GroupID: g.ID, UserID: bob.ID})
	db.Create(&models.GroupInvite{GroupID: g.ID, Email: "carol@example.com", InvitedByID: alice.ID})

	mustRun(t, "", "delete-user", "alice", "--yes")

	for name, model := range map[string]any{"tags": &models.Tag{}, "tag links": &models.ChoreTag{}, "hidden tags": &models.HiddenTag{}, "groups": &models.Group{}, "members": &models.GroupMember{}, "invites": &models.GroupInvite{}} {
		if n := rows(t, model, "1 = 1"); n != 0 {
			t.Errorf("%d %s left behind", n, name)
		}
	}
}
