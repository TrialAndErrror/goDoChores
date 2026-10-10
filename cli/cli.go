package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"goDoChores/database"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gorm.io/gorm"
)

// Commands returns the user-administration commands to attach to the root.
func Commands() []*cobra.Command {
	return []*cobra.Command{newCreateUserCmd(), newListUsersCmd(), newDeleteUserCmd()}
}

// openDB opens the database and makes sure its tables exist, so every command
// works on a brand-new install.
func openDB() (*gorm.DB, error) {
	if err := database.Open(); err != nil {
		return nil, err
	}
	if err := database.Migrate(); err != nil {
		return nil, err
	}
	return database.DB, nil
}

// readLine reads one line, trimmed. io.EOF with no text is returned as an error
// so callers can tell "no input" apart from an empty answer.
func readLine(in *bufio.Reader) (string, error) {
	line, err := in.ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || line == "") {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func prompt(in *bufio.Reader, out io.Writer, label string) (string, error) {
	fmt.Fprint(out, label)
	answer, err := readLine(in)
	if err != nil {
		return "", fmt.Errorf("no input for %q (use the flags for non-interactive use)", strings.TrimSuffix(strings.TrimSpace(label), ":"))
	}
	return answer, nil
}

// promptPassword reads a password. On a real terminal it hides what is typed
// and asks twice; otherwise (pipes, tests) it reads a plain line.
func promptPassword(cmd *cobra.Command, in *bufio.Reader) (string, error) {
	out := cmd.OutOrStdout()
	if f, ok := cmd.InOrStdin().(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		read := func(label string) (string, error) {
			fmt.Fprint(out, label)
			b, err := term.ReadPassword(int(f.Fd()))
			fmt.Fprintln(out)
			return string(b), err
		}
		first, err := read("Enter password: ")
		if err != nil {
			return "", err
		}
		second, err := read("Confirm password: ")
		if err != nil {
			return "", err
		}
		if first != second {
			return "", errors.New("passwords do not match")
		}
		return first, nil
	}
	return prompt(in, out, "Enter password: ")
}

func newCreateUserCmd() *cobra.Command {
	var username, email, password string
	cmd := &cobra.Command{
		Use:   "create-user",
		Short: "Create a user (or update an existing user's email and password)",
		Long: `Create a user. Any value not given as a flag is asked for interactively;
the password prompt is hidden when run in a terminal.

If a user with that username already exists, their email and password are updated.
Passing --password on the command line can leave it in your shell history.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB()
			if err != nil {
				return err
			}
			in := bufio.NewReader(cmd.InOrStdin())
			out := cmd.OutOrStdout()

			if !cmd.Flags().Changed("username") {
				if username, err = prompt(in, out, "Enter username: "); err != nil {
					return err
				}
			}
			if !cmd.Flags().Changed("email") {
				if email, err = prompt(in, out, "Enter email: "); err != nil {
					return err
				}
			}
			if !cmd.Flags().Changed("password") {
				if password, err = promptPassword(cmd, in); err != nil {
					return err
				}
			}

			created, err := SaveUser(db, username, email, password)
			if err != nil {
				return err
			}
			if created {
				fmt.Fprintf(out, "User created: %s\n", strings.TrimSpace(username))
			} else {
				fmt.Fprintf(out, "User %s already existed; email and password updated, and their existing logins were signed out\n", strings.TrimSpace(username))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&username, "username", "u", "", "username")
	cmd.Flags().StringVarP(&email, "email", "e", "", "email address")
	cmd.Flags().StringVarP(&password, "password", "p", "", "password (prefer the hidden prompt)")
	return cmd
}

func newListUsersCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "list-users",
		Short:        "List all users and how much data each has",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB()
			if err != nil {
				return err
			}
			users, err := ListUsers(db)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(users) == 0 {
				fmt.Fprintln(out, "No users.")
				return nil
			}
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tUSERNAME\tEMAIL\tCHORES\tREMINDERS\tTASKS\tCREATED")
			for _, u := range users {
				fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%d\t%d\t%s\n", u.ID, u.Username, u.Email, u.Chores, u.Reminders, u.Tasks, u.CreatedAt.Format("2006-01-02"))
			}
			return w.Flush()
		},
	}
}

func newDeleteUserCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete-user <username>",
		Short: "Permanently delete a user and all of their chores, reminders and tasks",
		Long: `Permanently delete a user and everything they own. This cannot be undone.

You are asked to type the username to confirm; pass --yes to skip the question.
Any logins the user currently has stop working immediately.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			summary, err := DescribeUser(db, args[0])
			if err != nil {
				return err
			}

			if !yes {
				fmt.Fprintf(out, "This will permanently delete user %q (%s) and their %d chores, %d reminders and %d tasks.\n",
					summary.Username, summary.Email, summary.Chores, summary.Reminders, summary.Tasks)
				answer, err := prompt(bufio.NewReader(cmd.InOrStdin()), out, fmt.Sprintf("Type %q to confirm: ", summary.Username))
				if err != nil || answer != summary.Username {
					return errors.New("aborted; nothing was deleted")
				}
			}

			if _, err := DeleteUser(db, summary.Username); err != nil {
				return err
			}
			fmt.Fprintf(out, "Deleted user %s and their %d chores, %d reminders and %d tasks\n",
				summary.Username, summary.Chores, summary.Reminders, summary.Tasks)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do not ask for confirmation")
	return cmd
}
