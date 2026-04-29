package work

import (
	"os"
	"path/filepath"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/work"
	"github.com/spf13/cobra"
)

func workspaceRoot(cmd *cobra.Command) (string, error) {
	cfgPath := cmd.Root().PersistentFlags().Lookup("config").Value.String()
	if cfgPath == "" {
		return os.Getwd()
	}
	absCfgPath, err := filepath.Abs(cfgPath)
	if err != nil {
		return "", err
	}
	return filepath.Dir(absCfgPath), nil
}

func workDir(root string) string {
	return work.Dir(root)
}

func workFilePath(root, name string) string {
	return work.FilePath(root, name)
}

func loadRepoConfig(cmd *cobra.Command) (*config.Config, string, error) {
	cfgPath := cmd.Root().PersistentFlags().Lookup("config").Value.String()
	if cfgPath == "" {
		cfgPath = "repos.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, "", err
	}
	return cfg, cfgPath, nil
}

