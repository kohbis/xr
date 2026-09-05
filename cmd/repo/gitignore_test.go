package repo

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newRootWithGlobalFlags builds the command tree confirmIgnoreWorkspace reads
// its flags from: --non-interactive and --yes live on the root command.
func newRootWithGlobalFlags(t *testing.T, nonInteractive, yes bool) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "xr"}
	root.PersistentFlags().Bool("non-interactive", false, "")
	root.PersistentFlags().Bool("yes", false, "")
	if nonInteractive {
		if err := root.PersistentFlags().Set("non-interactive", "true"); err != nil {
			t.Fatal(err)
		}
	}
	if yes {
		if err := root.PersistentFlags().Set("yes", "true"); err != nil {
			t.Fatal(err)
		}
	}
	child := &cobra.Command{Use: "gitignore"}
	root.AddCommand(child)
	return child
}

func TestConfirmIgnoreWorkspace_YesApplies(t *testing.T) {
	cmd := newRootWithGlobalFlags(t, true, true)

	ignore, err := confirmIgnoreWorkspace(cmd, "./repos")
	if err != nil {
		t.Fatalf("confirmIgnoreWorkspace() error = %v", err)
	}
	if !ignore {
		t.Error("--yes should add the workspace entry without prompting")
	}
}

func TestConfirmIgnoreWorkspace_NonInteractiveRequiresYes(t *testing.T) {
	cmd := newRootWithGlobalFlags(t, true, false)

	_, err := confirmIgnoreWorkspace(cmd, "./repos")
	if err == nil {
		t.Fatal("expected an error when prompting is impossible and --yes is absent")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error should name the flag to pass, got %q", err)
	}
}

// Without a terminal on stdin — a CI run with stdin redirected — the command
// must fail rather than read EOF and quietly decide "no", which is what
// reading stdin directly used to do.
func TestConfirmIgnoreWorkspace_NoTTYFailsInsteadOfAnsweringNo(t *testing.T) {
	// A regular file is not a character device, so the TTY check sees no
	// terminal. go test leaves stdin on /dev/null, which would look like one.
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	original := os.Stdin
	os.Stdin = f
	t.Cleanup(func() { os.Stdin = original })

	cmd := newRootWithGlobalFlags(t, false, false)

	if _, err := confirmIgnoreWorkspace(cmd, "./repos"); err == nil {
		t.Fatal("expected an error when stdin is not a terminal and --yes is absent")
	} else if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error should name the flag to pass, got %q", err)
	}
}
