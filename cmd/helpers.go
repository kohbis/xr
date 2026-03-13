package cmd

import (
	"path/filepath"

	"github.com/kohbis/xr/internal/config"
)

func loadConfig() (*config.Config, error) {
	cfgPath := cfgFile
	if cfgPath == "" {
		cfgPath = "repos.yaml"
	}
	return config.Load(cfgPath)
}

func resolveWorkspaceDir(cfg *config.Config) (string, error) {
	return filepath.Abs(cfg.Workspace)
}
