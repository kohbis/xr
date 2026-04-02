package output

import (
	"fmt"
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

func PrintRepoHeader(name string) {
	fmt.Printf("\n%s%s=== %s ===%s\n", colorBold, colorCyan, name, colorReset)
}

func PrintMatch(repo, file string, line int, content string, isContext bool) {
	if isContext {
		fmt.Printf("  %s%d%s-%s\n", colorDim, line, colorReset, content)
	} else {
		fmt.Printf("%s%s%s:%s%s%s:%s%d%s:%s\n",
			colorGreen, repo, colorReset,
			colorBlue, file, colorReset,
			colorYellow, line, colorReset,
			content,
		)
	}
}

func PrintMatchSimple(repo, file string, line int, content string, isContext bool) {
	if isContext {
		fmt.Printf("  %s%s%s/%s:%s%d%s-%s\n",
			colorDim, repo, colorReset,
			colorDim, colorReset,
			line,
			colorDim, colorReset,
		)
		_ = content
	} else {
		trimmed := strings.TrimSpace(content)
		fmt.Printf("%s%s%s/%s%s%s:%s%d%s: %s\n",
			colorGreen, repo, colorReset,
			colorBlue, file, colorReset,
			colorYellow, line, colorReset,
			trimmed,
		)
	}
}

func PrintWarning(msg string) {
	fmt.Printf("%swarning: %s%s\n", colorYellow, msg, colorReset)
}

func PrintError(msg string) {
	fmt.Printf("%serror: %s%s\n", colorRed, msg, colorReset)
}

func PrintSuccess(msg string) {
	fmt.Printf("%s%s%s\n", colorGreen, msg, colorReset)
}

func PrintDiffLine(line string) {
	if strings.HasPrefix(line, "+") {
		fmt.Printf("%s%s%s\n", colorGreen, line, colorReset)
	} else if strings.HasPrefix(line, "-") {
		fmt.Printf("%s%s%s\n", colorRed, line, colorReset)
	} else if strings.HasPrefix(line, "@@") {
		fmt.Printf("%s%s%s\n", colorCyan, line, colorReset)
	} else {
		fmt.Println(line)
	}
}

// PrintSyncHeader prints a repo header for sync operations.
func PrintSyncHeader(name, repoType string) {
	fmt.Printf("\n%s%s[%s]%s %s%s%s\n", colorBold, colorCyan, repoType, colorReset, colorBold, name, colorReset)
}

// PrintSyncSkip prints a skip message for repos that don't need syncing.
func PrintSyncSkip(reason string) {
	fmt.Printf("  %s⊘ %s%s\n", colorDim, reason, colorReset)
}

// PrintSyncOK prints a success message for a sync step.
func PrintSyncOK(msg string) {
	fmt.Printf("  %s✓ %s%s\n", colorGreen, msg, colorReset)
}

// PrintSyncAction prints an action being performed.
func PrintSyncAction(msg string) {
	fmt.Printf("  %s→%s %s\n", colorBlue, colorReset, msg)
}

// PrintSyncFail prints a failure message for a sync step.
func PrintSyncFail(msg string) {
	fmt.Printf("  %s✗ %s%s\n", colorRed, msg, colorReset)
}

// PrintSyncSummary prints the final summary of a sync operation.
func PrintSyncSummary(synced, skipped, failed int) {
	parts := []string{}
	if synced > 0 {
		parts = append(parts, fmt.Sprintf("%s%d synced%s", colorGreen, synced, colorReset))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%s%d skipped%s", colorDim, skipped, colorReset))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%s%d failed%s", colorRed, failed, colorReset))
	}
	fmt.Printf("\nDone: %s\n", strings.Join(parts, ", "))
}
