package repo

import (
	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/workspace"
)

// newWorkspace returns the workspace rooted at the config file's directory.
func newWorkspace(cfg *config.Config) *workspace.Workspace {
	return workspace.New(cfg.Root(), cfg)
}
