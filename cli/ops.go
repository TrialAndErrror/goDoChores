package cli

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"goDoChores/database"

	"github.com/spf13/cobra"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// sqliteHeader is the first 16 bytes of every SQLite database file.
const sqliteHeader = "SQLite format 3\x00"

// checkDatabaseFile verifies that path is a healthy SQLite database that looks
// like this app's (it has a users table). It opens the file read-only.
func checkDatabaseFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	header := make([]byte, len(sqliteHeader))
	_, readErr := io.ReadFull(f, header)
	f.Close()
	if readErr != nil || string(header) != sqliteHeader {
		return errors.New("not a SQLite database file")
	}

	db, err := gorm.Open(sqlite.Open("file:"+path+"?mode=ro"), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	var result string
	if err := db.Raw("PRAGMA integrity_check").Scan(&result).Error; err != nil {
		return fmt.Errorf("integrity check: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity check failed: %s", result)
	}
	var tables int
	if err := db.Raw("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'users'").Scan(&tables).Error; err != nil {
		return err
	}
	if tables == 0 {
		return errors.New("no users table: this does not look like a goDoChores database")
	}
	return nil
}

// BackupTo writes a consistent snapshot of the live database to path. Unlike
// copying the file, VACUUM INTO is safe while the server is writing.
func BackupTo(db *gorm.DB, path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := db.Exec("VACUUM INTO ?", path).Error; err != nil {
		os.Remove(path)
		return fmt.Errorf("backing up: %w", err)
	}
	if err := checkDatabaseFile(path); err != nil {
		os.Remove(path)
		return fmt.Errorf("backup failed verification: %w", err)
	}
	return nil
}

func newBackupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backup <file | ->",
		Short: "Write a consistent snapshot of the database (safe while the server runs)",
		Long: `Write a verified snapshot of the database to a file, or to standard output
when the file is "-". Status messages go to standard error so that standard
output carries only the database.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := database.Open(); err != nil {
				return err
			}
			defer database.Close()

			target := args[0]
			if target != "-" {
				if err := BackupTo(database.DB, target); err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Backup written to %s\n", target)
				return nil
			}

			// Stream: snapshot into a temporary file, then copy it out.
			dir, err := os.MkdirTemp("", "backup-")
			if err != nil {
				return err
			}
			defer os.RemoveAll(dir)
			tmp := filepath.Join(dir, "snapshot.db")
			if err := BackupTo(database.DB, tmp); err != nil {
				return err
			}
			f, err := os.Open(tmp)
			if err != nil {
				return err
			}
			defer f.Close()
			n, err := io.Copy(cmd.OutOrStdout(), f)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Backup streamed (%d bytes)\n", n)
			return nil
		},
	}
}

// RestoreFrom replaces the database file at dest with the database in src
// after verifying it. The old database is kept next to it with a
// .before-restore suffix, and stale journal files are removed so they cannot be
// replayed into the restored data. The server must not be running.
func RestoreFrom(src, dest string) (previous string, err error) {
	if err := checkDatabaseFile(src); err != nil {
		return "", fmt.Errorf("refusing to restore %s: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return "", err
	}

	// Copy next to the destination first so the final step is an atomic rename.
	staged, err := os.CreateTemp(filepath.Dir(dest), ".restore-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(staged.Name())
	in, err := os.Open(src)
	if err != nil {
		staged.Close()
		return "", err
	}
	_, copyErr := io.Copy(staged, in)
	in.Close()
	if closeErr := staged.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return "", copyErr
	}
	if err := os.Chmod(staged.Name(), 0o600); err != nil {
		return "", err
	}

	if _, statErr := os.Stat(dest); statErr == nil {
		previous = fmt.Sprintf("%s.before-restore-%s", dest, time.Now().Format("20060102-150405"))
		if err := os.Rename(dest, previous); err != nil {
			return "", err
		}
	}
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		os.Remove(dest + suffix)
	}
	if err := os.Rename(staged.Name(), dest); err != nil {
		return previous, err
	}
	return previous, nil
}

func newRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <file | ->",
		Short: "Replace the database with a backup (stop the server first)",
		Long: `Replace the database (DB_PATH) with the given backup, or with data read from
standard input when the file is "-". The backup is checked first. The previous
database is kept beside it as <db>.before-restore-<timestamp>.

The server must be stopped while this runs.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			src := args[0]
			if src == "-" {
				tmp, err := os.CreateTemp("", "restore-in-")
				if err != nil {
					return err
				}
				defer os.Remove(tmp.Name())
				if _, err := io.Copy(tmp, cmd.InOrStdin()); err != nil {
					tmp.Close()
					return err
				}
				if err := tmp.Close(); err != nil {
					return err
				}
				src = tmp.Name()
			}

			dest := database.Path()
			previous, err := RestoreFrom(src, dest)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Database restored to %s\n", dest)
			if previous != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Previous database kept as %s\n", previous)
			}
			return nil
		},
	}
}

// healthURL is where the container health check looks.
func healthURL() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	return "http://127.0.0.1:" + port + "/login"
}

func newHealthcheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "healthcheck",
		Short:        "Exit 0 if the local server answers (used by the container health check)",
		Hidden:       true,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Get(healthURL())
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("%s returned %s", healthURL(), resp.Status)
			}
			return nil
		},
	}
}
