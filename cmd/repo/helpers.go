package repo

import (
	"github.com/kohbis/xr/internal/config"
	"github.com/spf13/cobra"
)

func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	cfgPath := cmd.Root().PersistentFlags().Lookup("config").Value.String()
	if cfgPath == "" {
		cfgPath = "repos.yaml"
	}
	return config.Load(cfgPath)
}
