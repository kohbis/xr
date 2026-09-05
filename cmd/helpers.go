package cmd

import (
	"github.com/kohbis/xr/internal/config"
)

// loadConfig loads the config selected by --config.
func loadConfig() (*config.Config, error) {
	return config.LoadCommand(rootCmd)
}

// resolveWorkspaceDir returns the repositories directory, resolved relative to
// the config file so commands behave the same from any working directory.
func resolveWorkspaceDir(cfg *config.Config) (string, error) {
	return cfg.WorkspaceDir()
}
