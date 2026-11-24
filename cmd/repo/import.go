package repo

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/github"
	"github.com/kohbis/xr/internal/interactive"
	"github.com/kohbis/xr/internal/output"
	"github.com/spf13/cobra"
)

var (
	importDryRun bool
	importGitHub string
	importRepos  []string
	importProto  string
	importJSON   bool
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import repositories on disk or from GitHub into repos.yaml",
	Long: `Scan the workspace directory for repositories and add new ones to repos.yaml.

With --github OWNER, list the repositories of a GitHub user or organization
instead, and choose the ones to add from a prompt, or name them with --repo.
Each is
added as a clone of its default branch, with an ssh URL unless --protocol
https is given. Archived repositories and forks are not offered, nor
are repositories already in repos.yaml under any URL form; one whose name is
taken by another repository is skipped with a warning. Nothing is cloned: run
'xr repo sync --clone-missing' next. Without --repo, --github needs a terminal
for the prompt; --dry-run lists the candidates. A --repo name that is already
in repos.yaml is skipped, and one the owner does not have (or that is archived
or a fork) is an error. This
reads GitHub through the gh CLI, so it needs gh installed and logged in
(GH_HOST selects a GitHub Enterprise host), and fails when XR_NO_GH is set.

Lists newly discovered repos, then prompts for confirmation before writing repos.yaml.
Use --dry-run to print discoveries without writing or prompting.

For automation, use --yes to apply without prompting, or --non-interactive to fail
instead of waiting for input (requires --yes to write). --json prints the
repositories found (with --dry-run) or added as JSON and never prompts, so it
needs --dry-run or --yes, and -r with --github.

Examples:
  xr repo import --dry-run
  xr repo import --github acme --dry-run
  xr repo import --github acme
  xr repo import --github acme -r api -r web --yes && xr repo sync --clone-missing`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if importGitHub == "" && (len(importRepos) > 0 || cmd.Flags().Changed("protocol")) {
			return fmt.Errorf("--repo and --protocol require --github")
		}
		if importProto != github.ProtocolSSH && importProto != github.ProtocolHTTPS {
			return fmt.Errorf("--protocol must be ssh or https")
		}
		if importGitHub != "" {
			// Fail before touching anything when gh cannot be used, or when
			// the repositories could not be chosen.
			if err := github.Available(); err != nil {
				return err
			}
			if !importDryRun && len(importRepos) == 0 {
				shouldPrompt, err := interactive.ShouldPrompt(cmd)
				if err != nil {
					return err
				}
				if !shouldPrompt || importJSON {
					return fmt.Errorf("--github needs --repo to name the repositories without a prompt (use --dry-run to list them)")
				}
			}
		}

		if importJSON && !importDryRun && !interactive.Yes(cmd) {
			return fmt.Errorf("--json does not prompt, so writing repos.yaml requires --yes (or use --dry-run)")
		}

		cfgPath := config.CommandPath(cmd)
		cfg, err := config.Load(cfgPath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			// No config yet: scan next to where it will be written.
			absCfgPath, err := filepath.Abs(cfgPath)
			if err != nil {
				return err
			}
			cfg = &config.Config{Workspace: "./repos", Path: absCfgPath}
		}

		var (
			newRepos []config.Repository
			warnings []string
		)
		if importGitHub != "" {
			newRepos, warnings, err = githubRepos(cfg)
		} else {
			newRepos, warnings, err = scannedRepos(cfg)
		}
		if err != nil {
			return err
		}
		if !importJSON {
			for _, w := range warnings {
				output.PrintWarning(w)
			}
		}

		if len(newRepos) == 0 {
			if importJSON {
				return printImportJSON(nil, warnings)
			}
			fmt.Println("No new repositories found.")
			return nil
		}

		// Choosing from the prompt is the confirmation for --github.
		chosen := false
		if importGitHub != "" && !importDryRun && len(importRepos) == 0 {
			newRepos, err = chooseRepos(newRepos)
			if err != nil {
				return err
			}
			if len(newRepos) == 0 {
				fmt.Println("No repositories selected.")
				return nil
			}
			chosen = true
		}

		if importJSON && importDryRun {
			return printImportJSON(newRepos, warnings)
		}
		if !importJSON {
			fmt.Printf("Found %d new repo(s):\n", len(newRepos))
			for _, r := range newRepos {
				fmt.Printf("  + %-20s %-8s %s\n", r.Name, string(r.Type), r.Source)
			}
		}

		if importDryRun {
			return nil
		}

		if chosen || interactive.Yes(cmd) {
			// apply without prompting
		} else {
			shouldPrompt, err := interactive.ShouldPrompt(cmd)
			if err != nil {
				return err
			}
			if !shouldPrompt {
				return fmt.Errorf("non-interactive import requires --yes (or use --dry-run)")
			}

			fmt.Print("\nAdd these to repos.yaml? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		cfg.Repositories = append(cfg.Repositories, newRepos...)
		if err := config.Save(cfgPath, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		if importJSON {
			return printImportJSON(newRepos, warnings)
		}
		fmt.Printf("Added %d repo(s) to %s.\n", len(newRepos), cfgPath)
		return nil
	},
}

// importedRepo is one repository in the --json output.
type importedRepo struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Type   string `json:"type,omitempty"`
	Branch string `json:"branch,omitempty"`
	Path   string `json:"path"`
}

// printImportJSON prints the repositories found, which were also written to
// repos.yaml unless --dry-run is set. Warnings go in the output rather than to
// stderr.
func printImportJSON(repos []config.Repository, warnings []string) error {
	list := make([]importedRepo, 0, len(repos))
	for _, r := range repos {
		list = append(list, importedRepo{Name: r.Name, Source: r.Source, Type: string(r.Type), Branch: r.Branch, Path: r.Path})
	}
	if warnings == nil {
		warnings = []string{}
	}
	addedCount := 0
	if !importDryRun {
		addedCount = len(list)
	}
	return output.PrintJSON(output.CommandResult{
		Command: "repo import",
		Summary: map[string]int{"found": len(list), "added": addedCount},
		Data: map[string]any{
			"dry_run":  importDryRun,
			"repos":    list,
			"warnings": warnings,
		},
	})
}

// scannedRepos returns the repositories found in the workspace directory that
// repos.yaml does not list yet.
func scannedRepos(cfg *config.Config) ([]config.Repository, []string, error) {
	scan, err := newWorkspace(cfg).ScanRepos()
	if err != nil {
		return nil, nil, fmt.Errorf("scanning workspace: %w", err)
	}

	existing := make(map[string]struct{}, len(cfg.Repositories))
	for _, r := range cfg.Repositories {
		existing[r.Path] = struct{}{}
	}
	var newRepos []config.Repository
	for _, r := range scan.Repos {
		if _, ok := existing[r.Path]; !ok {
			newRepos = append(newRepos, r)
		}
	}
	return newRepos, scan.Warnings, nil
}

// githubRepos returns the repositories of the --github owner that repos.yaml
// does not list yet.
func githubRepos(cfg *config.Config) ([]config.Repository, []string, error) {
	listed, truncated, err := github.ListRepos(importGitHub)
	if err != nil {
		return nil, nil, fmt.Errorf("listing repositories of %s: %w", importGitHub, err)
	}
	var warnings []string
	if truncated {
		warnings = append(warnings, fmt.Sprintf("listing of %s stopped at %d repositories", importGitHub, len(listed)))
	}
	if len(importRepos) > 0 {
		listed, err = github.SelectByName(listed, importRepos)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", importGitHub, err)
		}
	}

	newRepos, conflicts := github.PlanImport(cfg.Repositories, listed, importProto)
	for _, c := range conflicts {
		warnings = append(warnings, "skipped "+c)
	}
	return newRepos, warnings, nil
}

// chooseRepos asks which of candidates to import and returns them in their
// listed order.
func chooseRepos(candidates []config.Repository) ([]config.Repository, error) {
	names := make([]string, 0, len(candidates))
	for _, r := range candidates {
		names = append(names, r.Name)
	}
	selected, err := promptMultiSelect("Select repos to import", names, 15)
	if err != nil {
		return nil, err
	}
	want := make(map[string]bool, len(selected))
	for _, n := range selected {
		want[n] = true
	}
	var chosen []config.Repository
	for _, r := range candidates {
		if want[r.Name] {
			chosen = append(chosen, r)
		}
	}
	return chosen, nil
}

func init() {
	importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "preview only, do not write")
	importCmd.Flags().BoolVar(&importJSON, "json", false, "output in JSON format (no prompts: needs --dry-run or --yes)")
	importCmd.Flags().StringVar(&importGitHub, "github", "", "list the repositories of a GitHub user or organization instead of scanning (uses gh)")
	importCmd.Flags().StringVar(&importProto, "protocol", github.ProtocolSSH, "with --github, clone URL protocol: ssh or https")
	importCmd.Flags().StringArrayVarP(&importRepos, "repo", "r", nil, "with --github, import this repository instead of choosing from a prompt (repeatable)")
}
