package repo

import (
	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/workspace"
	"github.com/spf13/cobra"
)

// loadConfig loads the config selected by --config. The returned config knows
// its own path, so workspace directories resolve relative to it.
func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	return config.LoadCommand(cmd)
}

// newWorkspace returns the workspace rooted at the config file's directory.
func newWorkspace(cfg *config.Config) *workspace.Workspace {
	return workspace.New(cfg.Root(), cfg)
}
