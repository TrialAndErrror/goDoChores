package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goDoChores/database"
	"goDoChores/models"

	"github.com/spf13/cobra"
)

// runSplit is run() with stdout and stderr kept apart, since `backup -` must
// put only the database on stdout.
func runSplit(t *testing.T, stdin []byte, args ...string) (stdout, stderr []byte, err error) {
	t.Helper()
	root := &cobra.Command{Use: "app", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(Commands()...)
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetIn(bytes.NewReader(stdin))
	root.SetArgs(args)
	err = root.Execute()
	return out.Bytes(), errOut.Bytes(), err
}

func dbPath() string { return os.Getenv("DB_PATH") }

func hasUser(t *testing.T, username string) bool {
	t.Helper()
	if err := database.Open(); err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var n int64
	database.DB.Model(&models.User{}).Where("username = ?", username).Count(&n)
	return n == 1
}

func TestBackupToFile(t *testing.T) {
	freshDB(t)
	mustRun(t, "", "create-user", "-u", "alice", "-e", "a@x.com", "-p", "pw")
	dest := filepath.Join(t.TempDir(), "nested", "dir", "snap.db")

	if _, stderr, err := runSplit(t, nil, "backup", dest); err != nil || !strings.Contains(string(stderr), dest) {
		t.Fatalf("backup failed: %v / %s", err, stderr)
	}
	if err := checkDatabaseFile(dest); err != nil {
		t.Fatalf("backup is not a valid database: %v", err)
	}

	// The snapshot has the data, and the live database is untouched.
	t.Setenv("DB_PATH", dest)
	if !hasUser(t, "alice") {
		t.Error("backup is missing alice")
	}

	// It will not overwrite an existing file.
	freshDB(t)
	if _, _, err := runSplit(t, nil, "backup", dest); err == nil {
		t.Error("backup should refuse to overwrite an existing file")
	}
}

func TestBackupToStdoutIsJustTheDatabase(t *testing.T) {
	freshDB(t)
	mustRun(t, "", "create-user", "-u", "alice", "-e", "a@x.com", "-p", "pw")

	stdout, stderr, err := runSplit(t, nil, "backup", "-")
	if err != nil {
		t.Fatalf("backup - failed: %v / %s", err, stderr)
	}
	if !bytes.HasPrefix(stdout, []byte(sqliteHeader)) {
		t.Fatalf("stdout does not start with a SQLite header (status text mixed in?): %.40q", stdout)
	}
	if !strings.Contains(string(stderr), "bytes") || bytes.Contains(stdout, []byte("Backup streamed")) {
		t.Errorf("status belongs on stderr only: stderr=%q", stderr)
	}

	snap := filepath.Join(t.TempDir(), "streamed.db")
	if err := os.WriteFile(snap, stdout, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkDatabaseFile(snap); err != nil {
		t.Errorf("streamed backup is not valid: %v", err)
	}
}

func TestBackupIsConsistentWhileTheDatabaseIsInUse(t *testing.T) {
	freshDB(t)
	mustRun(t, "", "create-user", "-u", "alice", "-e", "a@x.com", "-p", "pw")

	// Hold the live database open with a handle, as a running server would.
	if err := database.Open(); err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	for i := 0; i < 50; i++ {
		database.DB.Create(&models.Chore{Name: "c", UserID: 1})
	}
	dest := filepath.Join(t.TempDir(), "live.db")
	if err := BackupTo(database.DB, dest); err != nil {
		t.Fatalf("backup of an open database failed: %v", err)
	}
	if err := checkDatabaseFile(dest); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreReplacesTheDatabaseAndKeepsTheOldOne(t *testing.T) {
	freshDB(t)
	mustRun(t, "", "create-user", "-u", "alice", "-e", "a@x.com", "-p", "pw")
	backup, _, err := runSplit(t, nil, "backup", "-")
	if err != nil {
		t.Fatal(err)
	}

	// Now the live database changes: alice is replaced by bob.
	mustRun(t, "", "delete-user", "alice", "--yes")
	mustRun(t, "", "create-user", "-u", "bob", "-e", "b@x.com", "-p", "pw")
	// A leftover journal from a crash must not be replayed into the restore.
	if err := os.WriteFile(dbPath()+"-journal", []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr, err := runSplit(t, backup, "restore", "-")
	if err != nil {
		t.Fatalf("restore failed: %v / %s", err, stderr)
	}
	if !strings.Contains(string(stderr), "before-restore") {
		t.Errorf("should say where the previous database went: %s", stderr)
	}

	// Checked before opening the database: SQLite deletes an invalid journal
	// itself on open, which would hide a restore that forgot to.
	if _, err := os.Stat(dbPath() + "-journal"); err == nil {
		t.Error("stale journal file was left behind")
	}
	if !hasUser(t, "alice") || hasUser(t, "bob") {
		t.Error("restore did not bring back the backed-up data")
	}
	kept, _ := filepath.Glob(dbPath() + ".before-restore-*")
	if len(kept) != 1 {
		t.Fatalf("want exactly one kept copy of the old database, got %v", kept)
	}
	// The kept copy is the pre-restore state (bob, no alice).
	t.Setenv("DB_PATH", kept[0])
	if !hasUser(t, "bob") {
		t.Error("the kept copy should hold the data from before the restore")
	}
}

func TestRestoreRejectsBadInputAndLeavesTheDatabaseAlone(t *testing.T) {
	freshDB(t)
	mustRun(t, "", "create-user", "-u", "alice", "-e", "a@x.com", "-p", "pw")
	good, _, _ := runSplit(t, nil, "backup", "-")

	// A real, healthy SQLite database that is not ours: it has no users table.
	other := filepath.Join(t.TempDir(), "other.db")
	t.Setenv("DB_PATH", other)
	if err := database.Open(); err != nil {
		t.Fatal(err)
	}
	database.DB.Exec("CREATE TABLE unrelated (id integer)")
	database.Close()
	otherBytes, _ := os.ReadFile(other)

	// Back to a live database for the actual test.
	freshDB(t)
	mustRun(t, "", "create-user", "-u", "alice", "-e", "a@x.com", "-p", "pw")

	cases := map[string][]byte{
		"empty":              {},
		"text":               []byte("definitely not a database"),
		"truncated database": good[:len(good)/2],
		"wrong app":          otherBytes,
		"header only":        []byte(sqliteHeader),
	}
	for name, data := range cases {
		if _, _, err := runSplit(t, data, "restore", "-"); err == nil {
			t.Errorf("%s: restore should have been refused", name)
		}
	}
	if matches, _ := filepath.Glob(dbPath() + ".before-restore-*"); len(matches) != 0 {
		t.Errorf("a refused restore still moved the live database: %v", matches)
	}
	if !hasUser(t, "alice") {
		t.Error("a refused restore damaged the live database")
	}
}

func TestBackupDeleteRestoreCycleThroughTheCLI(t *testing.T) {
	freshDB(t)
	mustRun(t, "", "create-user", "-u", "alice", "-e", "a@x.com", "-p", "pw")
	backup, _, err := runSplit(t, nil, "backup", "-")
	if err != nil {
		t.Fatal(err)
	}

	// Wipe everything, as `nuke` would, then restore into a brand-new database.
	os.Remove(dbPath())
	if hasUser(t, "alice") {
		t.Fatal("setup: database should be empty")
	}
	os.Remove(dbPath())
	if _, _, err := runSplit(t, backup, "restore", "-"); err != nil {
		t.Fatalf("restore into an empty location failed: %v", err)
	}
	if !hasUser(t, "alice") {
		t.Error("alice did not survive the backup/restore cycle")
	}
	// ...and her password still works.
	mustRun(t, "", "create-user", "-u", "alice", "-e", "a@x.com", "-p", "pw2")
}

func TestHealthcheck(t *testing.T) {
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login" {
			t.Errorf("health check asked for %s", r.URL.Path)
		}
		w.WriteHeader(status)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	t.Setenv("PORT", u.Port())

	if _, _, err := runSplit(t, nil, "healthcheck"); err != nil {
		t.Errorf("healthy server reported unhealthy: %v", err)
	}
	status = http.StatusInternalServerError
	if _, _, err := runSplit(t, nil, "healthcheck"); err == nil {
		t.Error("a 500 should be unhealthy")
	}
	srv.Close()
	if _, _, err := runSplit(t, nil, "healthcheck"); err == nil {
		t.Error("an unreachable server should be unhealthy")
	}
}
