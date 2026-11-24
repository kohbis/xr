package shellcomp

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// fileDirectives are the directives that ask the shell for file or directory
// names. A positional argument that completes to paths keeps that completion
// rather than being replaced by flag names.
const fileDirectives = cobra.ShellCompDirectiveFilterFileExt | cobra.ShellCompDirectiveFilterDirs

// OfferFlags makes every runnable command under root without subcommands
// offer its flags when the word being completed is empty, so `xr repo list
// <TAB>` lists --json and the rest instead of falling back to file names.
// Cobra on its own offers flags only after a leading "-".
//
// Positional candidates come first and are kept. Flags are not offered after
// "--", where every word is an argument, nor when the positional argument
// completes to file or directory names.
func OfferFlags(root *cobra.Command) {
	for _, c := range root.Commands() {
		if c.Hidden || c.Name() == "help" {
			continue
		}
		if c.HasSubCommands() {
			OfferFlags(c)
			continue
		}
		c.ValidArgsFunction = withFlags(c.ValidArgsFunction)
	}
}

func withFlags(next cobra.CompletionFunc) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var comps []string
		directive := cobra.ShellCompDirectiveDefault
		if next != nil {
			comps, directive = next(cmd, args, toComplete)
		}
		if toComplete != "" || afterDash(completionWords()) || directive&fileDirectives != 0 {
			return comps, directive
		}
		flags := FlagCandidates(cmd)
		if len(flags) == 0 {
			return comps, directive
		}
		// With candidates to show, the shell must not fall back to file names.
		return append(comps, flags...), directive | cobra.ShellCompDirectiveNoFileComp
	}
}

// FlagCandidates returns the long names of the flags cmd accepts, as
// "--name\tusage", in the order cobra lists them after "--". Hidden and
// deprecated flags are left out, and so is a flag already given unless it
// can be repeated.
func FlagCandidates(cmd *cobra.Command) []string {
	var out []string
	add := func(f *pflag.Flag) {
		if f.Hidden || f.Deprecated != "" {
			return
		}
		if f.Changed && !repeatable(f) {
			return
		}
		out = append(out, "--"+f.Name+"\t"+f.Usage)
	}
	cmd.NonInheritedFlags().VisitAll(add)
	cmd.InheritedFlags().VisitAll(add)
	return out
}

func repeatable(f *pflag.Flag) bool {
	t := f.Value.Type()
	return strings.Contains(t, "Slice") || strings.Contains(t, "Array")
}

// completionWords returns the words of the command line being completed,
// without the word under the cursor. It reads the request cobra's hidden
// completion command receives, `xr __complete <words...> <toComplete>`.
//
// cmd.ArgsLenAtDash cannot answer whether "--" was typed: while completing,
// cobra parses the flags once with an extra "--" appended and pflag does not
// reset the position on the second parse.
var completionWords = func() []string {
	for i, a := range os.Args {
		if a == cobra.ShellCompRequestCmd || a == cobra.ShellCompNoDescRequestCmd {
			words := os.Args[i+1:]
			if len(words) == 0 {
				return nil
			}
			return words[:len(words)-1]
		}
	}
	return nil
}

// afterDash reports whether the words contain "--", after which every word is
// a positional argument.
func afterDash(words []string) bool {
	for _, w := range words {
		if w == "--" {
			return true
		}
	}
	return false
}
