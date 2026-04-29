package work

import (
	"bytes"
	"fmt"
	"os"

	"github.com/kohbis/xr/internal/work"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var initForce bool

var initCmd = &cobra.Command{
	Use:   "init <name>",
	Short: "Create a work plan from repos.yaml",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		root, err := workspaceRoot(cmd)
		if err != nil {
			return err
		}
		dir := workDir(root)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating work dir: %w", err)
		}

		outPath := workFilePath(root, name)
		if _, err := work.SafeFilePath(root, name); err != nil {
			return err
		}
		if !initForce {
			if _, err := os.Stat(outPath); err == nil {
				return fmt.Errorf("work plan already exists: %s (use --force to overwrite)", outPath)
			}
		}

		cfg, cfgPath, err := loadRepoConfig(cmd)
		if err != nil {
			return err
		}
		_ = cfgPath

		// Initialize with repo names only. Add `branch` later only when needed.
		repos := make([]work.Repo, 0, len(cfg.Repositories))
		for _, r := range cfg.Repositories {
			repos = append(repos, work.Repo{
				Name: r.Name,
			})
		}
		out := work.File{
			Name:  name,
			Repos: repos,
		}
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(&out); err != nil {
			_ = enc.Close()
			return fmt.Errorf("marshaling work plan: %w", err)
		}
		if err := enc.Close(); err != nil {
			return fmt.Errorf("marshaling work plan: %w", err)
		}
		var outBuf bytes.Buffer
		outBuf.WriteString("# Keep only repos you need.\n")
		outBuf.WriteString("# To switch branches with `xr repo sync --work <name> --apply`, set `branch` per repo.\n\n")
		outBuf.Write(buf.Bytes())
		data := outBuf.Bytes()

		tmp := outPath + ".tmp"
		if err := os.WriteFile(tmp, data, 0644); err != nil {
			return fmt.Errorf("writing work plan: %w", err)
		}
		if err := os.Rename(tmp, outPath); err != nil {
			return fmt.Errorf("saving work plan: %w", err)
		}

		fmt.Println(outPath)
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite if the work plan already exists")
}

