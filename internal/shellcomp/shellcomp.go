package shellcomp

import (
	"slices"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/spf13/cobra"
)

// CompleteRepoNames completes positional arguments with repository names from repos.yaml.
func CompleteRepoNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	names := RepoNameCandidates(config.CommandPath(cmd), args, toComplete)
	if len(names) == 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return names, cobra.ShellCompDirectiveNoFileComp
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

// CompleteTags completes the --tag flag with distinct tags from repos.yaml.
func CompleteTags(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	tags := TagCandidates(config.CommandPath(cmd), givenTags(cmd), toComplete)
	if len(tags) == 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return tags, cobra.ShellCompDirectiveNoFileComp
}

// givenTags returns the tags already passed to this command's --tag flag.
func givenTags(cmd *cobra.Command) []string {
	if cmd == nil {
		return nil
	}
	tags, err := cmd.Flags().GetStringArray("tag")
	if err != nil {
		return nil
	}
	return tags
}

// TagCandidates returns distinct tags from the config at cfgPath that match
// prefix and are not listed in exclude. The result is sorted.
func TagCandidates(cfgPath string, exclude []string, prefix string) []string {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil
	}
	excludeSet := make(map[string]struct{}, len(exclude))
	for _, tag := range exclude {
		excludeSet[tag] = struct{}{}
	}
	seen := make(map[string]struct{})
	var tags []string
	for _, r := range cfg.Repositories {
		for _, tag := range r.Tags {
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			if _, skip := excludeSet[tag]; skip {
				continue
			}
			if strings.HasPrefix(tag, prefix) {
				tags = append(tags, tag)
			}
		}
	}
	slices.Sort(tags)
	return tags
}
