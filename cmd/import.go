package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/workspace"
	"github.com/spf13/cobra"
)

var importDryRun bool

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import existing repositories into repos.yaml",
	Long:  `Scan the workspace directory for repositories and add new ones to repos.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := cfgFile
		if cfgPath == "" {
			cfgPath = "repos.yaml"
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				cfg = &config.Config{Workspace: "./repos"}
			} else {
				return err
			}
		}

		wsDir, err := resolveWorkspaceDir(cfg)
		if err != nil {
			return err
		}

		ws := workspace.New(filepath.Dir(wsDir), cfg)
		found, err := ws.ScanRepos()
		if err != nil {
			return fmt.Errorf("scanning workspace: %w", err)
		}

		existing := make(map[string]struct{}, len(cfg.Repositories))
		for _, r := range cfg.Repositories {
			existing[r.Path] = struct{}{}
		}

		var newRepos []config.Repository
		for _, r := range found {
			if _, ok := existing[r.Path]; !ok {
				newRepos = append(newRepos, r)
			}
		}

		if len(newRepos) == 0 {
			fmt.Println("No new repositories found.")
			return nil
		}

		fmt.Printf("Found %d new repo(s):\n", len(newRepos))
		for _, r := range newRepos {
			fmt.Printf("  + %-20s %-8s %s\n", r.Name, string(r.Type), r.Source)
		}

		if importDryRun {
			return nil
		}

		fmt.Print("\nAdd these to repos.yaml? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" {
			fmt.Println("Aborted.")
			return nil
		}

		cfg.Repositories = append(cfg.Repositories, newRepos...)
		if err := config.Save(cfgPath, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Printf("Added %d repo(s) to %s.\n", len(newRepos), cfgPath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
	importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "preview only, do not write")
}
