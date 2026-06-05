package interactive

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestFlagBoolWithoutRegistration(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	if NonInteractive(cmd) || Yes(cmd) {
		t.Fatal("expected false when flags are not registered")
	}
}

func TestNonInteractiveAndYes(t *testing.T) {
	root := &cobra.Command{Use: "xr"}
	root.PersistentFlags().Bool("non-interactive", false, "")
	root.PersistentFlags().Bool("yes", false, "")

	if err := root.PersistentFlags().Set("non-interactive", "true"); err != nil {
		t.Fatal(err)
	}
	if err := root.PersistentFlags().Set("yes", "true"); err != nil {
		t.Fatal(err)
	}

	child := &cobra.Command{Use: "repo"}
	root.AddCommand(child)

	if !NonInteractive(child) {
		t.Fatal("expected non-interactive true")
	}
	if !Yes(child) {
		t.Fatal("expected yes true")
	}

	shouldPrompt, err := ShouldPrompt(child)
	if err != nil {
		t.Fatal(err)
	}
	if shouldPrompt {
		t.Fatal("expected shouldPrompt false when --non-interactive is set")
	}
}
