package repo

import (
	"path/filepath"

	"github.com/kohbis/xr/internal/config"
	"github.com/spf13/cobra"
)

func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	cfg, _, err := loadConfigWithPath(cmd)
	return cfg, err
}

func loadConfigWithPath(cmd *cobra.Command) (*config.Config, string, error) {
	cfgPath := cmd.Root().PersistentFlags().Lookup("config").Value.String()
	if cfgPath == "" {
		cfgPath = "repos.yaml"
	}
	absCfgPath, err := filepath.Abs(cfgPath)
	if err != nil {
		return nil, "", err
	}
	cfg, err := config.Load(absCfgPath)
	if err != nil {
		return nil, "", err
	}
	return cfg, absCfgPath, nil
}

func resolveWorkspaceDirFromConfigPath(cfgPath string, cfg *config.Config) (string, error) {
	cfgDir := filepath.Dir(cfgPath)
	return filepath.Abs(filepath.Clean(filepath.Join(cfgDir, cfg.Workspace)))
}
