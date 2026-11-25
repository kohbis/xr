package shellcomp

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestRoot builds a small command tree with the shapes OfferFlags handles.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "xr"}
	root.PersistentFlags().Bool("yes", false, "confirm")

	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, Run: func(*cobra.Command, []string) {}}
	list.Flags().Bool("json", false, "json output")
	list.Flags().StringArray("repo", nil, "repos")
	list.Flags().Bool("secret", false, "hidden")
	_ = list.Flags().MarkHidden("secret")

	tree := &cobra.Command{
		Use: "tree", Run: func(*cobra.Command, []string) {},
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return []string{"alpha"}, cobra.ShellCompDirectiveNoFileComp
		},
	}

	initCmd := &cobra.Command{
		Use: "init", Run: func(*cobra.Command, []string) {},
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveFilterDirs
		},
	}
	initCmd.Flags().String("file", "", "config path")

	exec := &cobra.Command{Use: "exec", Run: func(*cobra.Command, []string) {}}
	exec.Flags().Bool("fail-fast", false, "stop early")

	repo := &cobra.Command{Use: "repo"}
	repo.AddCommand(list)

	root.AddCommand(repo, tree, initCmd, exec)
	OfferFlags(root)
	return root
}

// complete runs cobra's completion request for words followed by toComplete
// and returns the candidate names (descriptions stripped) and the directive line.
func complete(t *testing.T, words []string, toComplete string) ([]string, string) {
	t.Helper()
	orig := completionWords
	completionWords = func() []string { return words }
	t.Cleanup(func() { completionWords = orig })

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append(append([]string{cobra.ShellCompRequestCmd}, words...), toComplete))
	if err := root.Execute(); err != nil {
		t.Fatalf("completion failed: %v", err)
	}
	var names []string
	var directive string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if strings.HasPrefix(line, ":") {
			directive = line
			continue
		}
		name, _, _ := strings.Cut(line, "\t")
		names = append(names, name)
	}
	return names, directive
}

func TestOfferFlags(t *testing.T) {
	noFile := ":4"
	tests := []struct {
		name       string
		words      []string
		toComplete string
		want       []string
		directive  string
	}{
		{
			name:      "leaf command lists its flags then inherited ones",
			words:     []string{"repo", "list"},
			want:      []string{"--help", "--json", "--repo", "--yes"},
			directive: noFile,
		},
		{
			name:      "a flag already given is left out unless repeatable",
			words:     []string{"repo", "list", "--json", "--repo", "a"},
			want:      []string{"--help", "--repo", "--yes"},
			directive: noFile,
		},
		{
			name:      "positional candidates come first",
			words:     []string{"tree"},
			want:      []string{"alpha", "--help", "--yes"},
			directive: noFile,
		},
		{
			name:      "parent command still lists subcommands only",
			words:     []string{"repo"},
			want:      []string{"list"},
			directive: noFile,
		},
		{
			name:       "a partial word completes positionals as before",
			words:      []string{"exec"},
			toComplete: "x",
			want:       nil,
			directive:  ":0",
		},
		{
			name:      "no flags after --",
			words:     []string{"exec", "--"},
			want:      nil,
			directive: ":0",
		},
		{
			name:      "directory completion is kept",
			words:     []string{"init"},
			want:      nil,
			directive: ":16",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, directive := complete(t, tt.words, tt.toComplete)
			// cobra adds its own help command at the root; ignore it.
			got = slices.DeleteFunc(got, func(s string) bool { return s == "help" || s == "completion" })
			if !slices.Equal(got, tt.want) {
				t.Errorf("candidates = %v, want %v", got, tt.want)
			}
			if directive != tt.directive {
				t.Errorf("directive = %s, want %s", directive, tt.directive)
			}
		})
	}
}

func TestAfterDash(t *testing.T) {
	if afterDash([]string{"exec", "-r", "a"}) {
		t.Error("no -- reported as after dash")
	}
	if !afterDash([]string{"exec", "--", "ls"}) {
		t.Error("-- not detected")
	}
}
