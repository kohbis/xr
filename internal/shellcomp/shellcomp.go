package shellcomp

import (
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/spf13/cobra"
)

// CompleteRepoNames completes positional arguments with repository names from repos.yaml.
func CompleteRepoNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	names := RepoNameCandidates(configPath(cmd), args, toComplete)
	if len(names) == 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func configPath(cmd *cobra.Command) string {
	p := cmd.Root().PersistentFlags().Lookup("config").Value.String()
	if p == "" {
		return "repos.yaml"
	}
	return p
}

// RepoNameCandidates returns names in cfg that match prefix and are not listed in exclude.
func RepoNameCandidates(cfgPath string, exclude []string, prefix string) []string {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil
	}
	excludeSet := make(map[string]struct{}, len(exclude))
	for _, a := range exclude {
		excludeSet[a] = struct{}{}
	}
	var names []string
	for _, r := range cfg.Repositories {
		if _, skip := excludeSet[r.Name]; skip {
			continue
		}
		if strings.HasPrefix(r.Name, prefix) {
			names = append(names, r.Name)
		}
	}
	return names
}
