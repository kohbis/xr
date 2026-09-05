package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/doctor"
	"github.com/kohbis/xr/internal/exitcode"
	"github.com/kohbis/xr/internal/output"
	"github.com/spf13/cobra"
)

var doctorJSON bool

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Short:   "Check the tools and workspace xr needs",
	GroupID: "meta",
	Long: `Check that the environment xr depends on is in place: the external tools it
shells out to (git, diff, and optionally ripgrep), the repos.yaml it would use,
and whether the workspace has been materialized.

Exits non-zero when something is broken — a required tool missing from PATH, or
a repos.yaml that cannot be read. A workspace that has not been created yet, or
a missing ripgrep, is a warning and exits 0, because commands still work.

Examples:
  xr doctor
  xr doctor --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		report := doctor.Run(config.CommandPath(cmd))

		if doctorJSON {
			result := output.CommandResult{
				Command: "doctor",
				Summary: map[string]int{
					"ok":       report.OK,
					"warnings": report.Warnings,
					"failed":   report.Failed,
				},
				Data: map[string]any{"checks": report.Checks},
			}
			if err := output.PrintJSON(result); err != nil {
				return err
			}
			return doctorExitCode(cmd, report)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, c := range report.Checks {
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", doctorMark(c.Status), c.Name, c.Detail); err != nil {
				return err
			}
			if c.Hint != "" {
				if _, err := fmt.Fprintf(w, "\t\t%s\n", output.Dim("→ "+c.Hint)); err != nil {
					return err
				}
			}
		}
		if err := w.Flush(); err != nil {
			return err
		}
		fmt.Printf("\n%s\n", doctorSummary(report))

		return doctorExitCode(cmd, report)
	},
}

// doctorSummary avoids output.PrintActionSummary, whose middle count is
// labelled "skipped": nothing is skipped here, and a warning is not a failure.
func doctorSummary(report doctor.Report) string {
	parts := []string{fmt.Sprintf("%d ok", report.OK)}
	if report.Warnings > 0 {
		parts = append(parts, output.Dim(fmt.Sprintf("%d %s", report.Warnings, plural(report.Warnings, "warning"))))
	}
	if report.Failed > 0 {
		parts = append(parts, output.Red(fmt.Sprintf("%d failed", report.Failed)))
	}
	return "Done: " + strings.Join(parts, ", ")
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// doctorMark renders a check's status. Warnings keep the exit status at 0, so
// they are marked differently from failures.
func doctorMark(status string) string {
	switch status {
	case doctor.StatusFailed:
		return output.Red("✗")
	case doctor.StatusWarning:
		return "!"
	default:
		return "✓"
	}
}

// doctorExitCode makes a broken environment gate a pipeline without parsing the
// output. Warnings do not fail: they describe a workspace that is not set up
// yet, which is not an error.
func doctorExitCode(cmd *cobra.Command, report doctor.Report) error {
	if report.Failed == 0 {
		return nil
	}
	return exitcode.Failed(cmd)
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "output in JSON format")
}
