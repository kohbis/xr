package interactive

import (
	"os"

	"github.com/spf13/cobra"
)

func stdinIsTTY() (bool, error) {
	in, err := os.Stdin.Stat()
	if err != nil {
		return false, err
	}
	return (in.Mode() & os.ModeCharDevice) != 0, nil
}

func flagBool(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}
	v, err := cmd.Root().PersistentFlags().GetBool(name)
	if err != nil {
		return false
	}
	return v
}

// NonInteractive reports whether --non-interactive was set on the root command.
func NonInteractive(cmd *cobra.Command) bool {
	return flagBool(cmd, "non-interactive")
}

// Yes reports whether --yes was set on the root command.
func Yes(cmd *cobra.Command) bool {
	return flagBool(cmd, "yes")
}

// ShouldPrompt returns true when the command may show interactive prompts.
// It is false when --non-interactive is set or stdin is not a TTY.
func ShouldPrompt(cmd *cobra.Command) (bool, error) {
	if NonInteractive(cmd) {
		return false, nil
	}
	return stdinIsTTY()
}
