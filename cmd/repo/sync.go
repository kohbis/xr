package repo

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/output"
	"github.com/kohbis/xr/internal/shellcomp"
	"github.com/kohbis/xr/internal/work"
	"github.com/kohbis/xr/internal/workspace"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync [repo...]",
	Short: "Sync repositories to match repos.yaml configuration",
	Long: `Synchronize repositories to match the configuration in repos.yaml.

By default this command previews actions only. Use --apply to run git operations.

Sync options (combine with --apply to execute):
  (none)           switch branches only
  --update         fetch and pull from remote
  --submodules     update submodules recursively (combine with --update for a full remote sync)

Always switches to the branch in repos.yaml (or work plan override with --work).

Scope with --work <name> (from .xr/work/<name>.yaml) instead of repo args.
Use --allow-dirty to proceed on dirty repos without prompting (recommended without a TTY).

Without arguments, syncs all repositories. Specify repo names to sync only those.

Examples:
  # Preview branch checkout (no network)
  xr repo sync

  # Execute branch checkout
  xr repo sync --apply

  # Fetch, checkout, and pull
  xr repo sync --update --apply

  # Fetch, pull, and update submodules
  xr repo sync --update --submodules --apply

  # Apply a work plan
  xr repo sync --work example --apply`,
	ValidArgsFunction: shellcomp.CompleteRepoNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSync(cmd, args)
	},
}

func runSync(cmd *cobra.Command, args []string) error {
	fetch, pull, submod := effectiveSyncNetwork()
	if err := validateSyncFlags(fetch); err != nil {
		return err
	}

	cfg, cfgPath, err := loadConfigWithPath(cmd)
	if err != nil {
		return err
	}

	if syncWork != "" && len(args) > 0 {
		return fmt.Errorf("cannot combine --work with explicit repo args")
	}

	// If --work is provided, scope sync to those repos and override branch where specified.
	repoArgs := args
	if syncWork != "" {
		root := filepath.Dir(cfgPath)
		workPath, err := work.SafeFilePath(root, syncWork)
		if err != nil {
			return err
		}
		wf, err := work.Load(workPath)
		if err != nil {
			return err
		}
		allowed := map[string]string{} // repo -> branch override (may be empty)
		for _, r := range wf.Repos {
			allowed[r.Name] = r.Branch
		}

		// Copy & filter config to avoid mutating shared state.
		cfgCopy := *cfg
		cfgCopy.Repositories = make([]config.Repository, 0, len(allowed))
		known := map[string]struct{}{}
		for _, r := range cfg.Repositories {
			known[r.Name] = struct{}{}
			if b, ok := allowed[r.Name]; ok {
				r.Branch = b // may be empty = do not checkout
				cfgCopy.Repositories = append(cfgCopy.Repositories, r)
			}
		}
		var unknown []string
		for name := range allowed {
			if _, ok := known[name]; !ok {
				unknown = append(unknown, name)
			}
		}
		if len(unknown) > 0 {
			return fmt.Errorf("work plan contains unknown repos: %s", strings.Join(unknown, ", "))
		}
		cfg = &cfgCopy
		repoArgs = nil // operate on all repos in cfgCopy (already filtered)
	}

	if syncApply {
		fmt.Printf("Syncing workspace...\n")
	} else {
		fmt.Printf("Previewing workspace sync (no changes will be made).\n")
	}

	ws := workspace.New(filepath.Dir(cfgPath), cfg)
	isTTY, err := isInteractiveTTY()
	if err != nil {
		return err
	}

	proceedAllDirty := false
	opts := workspace.SyncOptions{
		Pull:   pull,
		Fetch:  fetch,
		Prune:  syncPrune,
		Submod: submod,
		DryRun: !syncApply,

		AllowDirty:            syncDirty,
		CreateBranchIfMissing: syncCreateBranchIfMissing,
	}
	if isTTY && !opts.AllowDirty {
		opts.ConfirmDirty = func(repo config.Repository, reason string) (bool, error) {
			if proceedAllDirty {
				return true, nil
			}
			choice, err := promptSelect(nil, fmt.Sprintf("%s: %s", repo.Name, reason), []string{"Skip", "Proceed", "Proceed all"}, 10, false)
			if err != nil {
				return false, err
			}
			switch choice {
			case 0:
				return false, nil
			case 1:
				return true, nil
			case 2:
				proceedAllDirty = true
				return true, nil
			default:
				return false, nil
			}
		}
	}
	if isTTY && syncApply {
		opts.ConfirmCheckout = func(repo config.Repository, fromBranch, toBranch string) (bool, error) {
			// If fromBranch is empty, it likely means a detached HEAD; still confirm.
			label := fmt.Sprintf("%s: switch %q → %q", repo.Name, fromBranch, toBranch)
			return promptYesNoSelect(label, true)
		}
	}

	result, err := ws.Sync(repoArgs, opts)
	if err != nil {
		return fmt.Errorf("syncing workspace: %w", err)
	}

	if opts.DryRun {
		fmt.Printf("\nPreview done: %d repo(s)\n", result.Skipped)
		fmt.Printf("To execute: rerun the same command with --apply\n")
		if result.Failed > 0 {
			fmt.Printf("Preview failures: %d\n", result.Failed)
		}
		return nil
	}
	output.PrintSyncSummary(result.Synced, result.Skipped, result.Failed)
	return nil
}

func init() {
	RegisterSyncFlags(syncCmd)
}
