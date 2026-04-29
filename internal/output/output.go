package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

var colorEnabled = true

type RepoResult struct {
	Name    string         `json:"name"`
	Status  string         `json:"status"`
	Error   string         `json:"error,omitempty"`
	Metrics map[string]int `json:"metrics,omitempty"`
}

type CommandResult struct {
	Command string         `json:"command"`
	Summary map[string]int `json:"summary,omitempty"`
	Repos   []RepoResult   `json:"repos,omitempty"`
	Data    any            `json:"data,omitempty"`
}

func SetColorEnabled(enabled bool) {
	colorEnabled = enabled
}

func c(code string) string {
	if !colorEnabled {
		return ""
	}
	return code
}

func PrintJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func WriteJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func PrintRepoHeader(name string) {
	fmt.Printf("\n%s%s=== %s ===%s\n", c(colorBold), c(colorCyan), name, c(colorReset))
}

func PrintMatch(repo, file string, line int, content string, isContext bool) {
	if isContext {
		fmt.Printf("  %s%d%s-%s\n", c(colorDim), line, c(colorReset), content)
	} else {
		fmt.Printf("%s%s%s:%s%s%s:%s%d%s:%s\n",
			c(colorGreen), repo, c(colorReset),
			c(colorBlue), file, c(colorReset),
			c(colorYellow), line, c(colorReset),
			content,
		)
	}
}

func PrintMatchSimple(repo, file string, line int, content string, isContext bool) {
	if isContext {
		fmt.Printf("  %s%s%s/%s:%s%d%s-%s\n",
			c(colorDim), repo, c(colorReset),
			c(colorDim), c(colorReset),
			line,
			c(colorDim), c(colorReset),
		)
		_ = content
	} else {
		trimmed := strings.TrimSpace(content)
		fmt.Printf("%s%s%s/%s%s%s:%s%d%s: %s\n",
			c(colorGreen), repo, c(colorReset),
			c(colorBlue), file, c(colorReset),
			c(colorYellow), line, c(colorReset),
			trimmed,
		)
	}
}

func PrintWarning(msg string) {
	fmt.Printf("%swarning: %s%s\n", c(colorYellow), msg, c(colorReset))
}

func PrintError(msg string) {
	fmt.Printf("%serror: %s%s\n", c(colorRed), msg, c(colorReset))
}

func PrintSuccess(msg string) {
	fmt.Printf("%s%s%s\n", c(colorGreen), msg, c(colorReset))
}

func PrintDiffLine(line string) {
	if strings.HasPrefix(line, "+") {
		fmt.Printf("%s%s%s\n", c(colorGreen), line, c(colorReset))
	} else if strings.HasPrefix(line, "-") {
		fmt.Printf("%s%s%s\n", c(colorRed), line, c(colorReset))
	} else if strings.HasPrefix(line, "@@") {
		fmt.Printf("%s%s%s\n", c(colorCyan), line, c(colorReset))
	} else {
		fmt.Println(line)
	}
}

// PrintSyncHeader prints a repo header for sync operations.
func PrintSyncHeader(name, repoType string) {
	fmt.Printf("\n%s%s[%s]%s %s%s%s\n", c(colorBold), c(colorCyan), repoType, c(colorReset), c(colorBold), name, c(colorReset))
}

// PrintSyncSkip prints a skip message for repos that don't need syncing.
func PrintSyncSkip(reason string) {
	fmt.Printf("  %s⊘ %s%s\n", c(colorDim), reason, c(colorReset))
}

// PrintSyncOK prints a success message for a sync step.
func PrintSyncOK(msg string) {
	fmt.Printf("  %s✓ %s%s\n", c(colorGreen), msg, c(colorReset))
}

// PrintSyncAction prints an action being performed.
func PrintSyncAction(msg string) {
	fmt.Printf("  %s→%s %s\n", c(colorBlue), c(colorReset), msg)
}

// PrintSyncFail prints a failure message for a sync step.
func PrintSyncFail(msg string) {
	fmt.Printf("  %s✗ %s%s\n", c(colorRed), msg, c(colorReset))
}

// PrintSyncSummary prints the final summary of a sync operation.
func PrintSyncSummary(synced, skipped, failed int) {
	parts := []string{}
	if synced > 0 {
		parts = append(parts, fmt.Sprintf("%s%d synced%s", c(colorGreen), synced, c(colorReset)))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%s%d skipped%s", c(colorDim), skipped, c(colorReset)))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%s%d failed%s", c(colorRed), failed, c(colorReset)))
	}
	fmt.Printf("\nDone: %s\n", strings.Join(parts, ", "))
}
