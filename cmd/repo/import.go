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
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import existing repositories into repos.yaml",
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
instead of waiting for input (requires --yes to write).

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
				if !shouldPrompt {
					return fmt.Errorf("--github needs --repo to name the repositories without a terminal (use --dry-run to list them)")
				}
			}
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

		var newRepos []config.Repository
		if importGitHub != "" {
			newRepos, err = githubRepos(cfg)
		} else {
			newRepos, err = scannedRepos(cfg)
		}
		if err != nil {
			return err
		}

		if len(newRepos) == 0 {
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

		fmt.Printf("Found %d new repo(s):\n", len(newRepos))
		for _, r := range newRepos {
			fmt.Printf("  + %-20s %-8s %s\n", r.Name, string(r.Type), r.Source)
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

		fmt.Printf("Added %d repo(s) to %s.\n", len(newRepos), cfgPath)
		return nil
	},
}

// scannedRepos returns the repositories found in the workspace directory that
// repos.yaml does not list yet.
func scannedRepos(cfg *config.Config) ([]config.Repository, error) {
	scan, err := newWorkspace(cfg).ScanRepos()
	if err != nil {
		return nil, fmt.Errorf("scanning workspace: %w", err)
	}
	for _, warning := range scan.Warnings {
		output.PrintWarning(warning)
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
	return newRepos, nil
}

// githubRepos returns the repositories of the --github owner that repos.yaml
// does not list yet.
func githubRepos(cfg *config.Config) ([]config.Repository, error) {
	listed, truncated, err := github.ListRepos(importGitHub)
	if err != nil {
		return nil, fmt.Errorf("listing repositories of %s: %w", importGitHub, err)
	}
	if truncated {
		output.PrintWarning(fmt.Sprintf("listing of %s stopped at %d repositories", importGitHub, len(listed)))
	}
	if len(importRepos) > 0 {
		listed, err = github.SelectByName(listed, importRepos)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", importGitHub, err)
		}
	}

	newRepos, conflicts := github.PlanImport(cfg.Repositories, listed, importProto)
	for _, c := range conflicts {
		output.PrintWarning("skipped " + c)
	}
	return newRepos, nil
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
	importCmd.Flags().StringVar(&importGitHub, "github", "", "list the repositories of a GitHub user or organization instead of scanning (uses gh)")
	importCmd.Flags().StringVar(&importProto, "protocol", github.ProtocolSSH, "with --github, clone URL protocol: ssh or https")
	importCmd.Flags().StringArrayVarP(&importRepos, "repo", "r", nil, "with --github, import this repository instead of choosing from a prompt (repeatable)")
}
