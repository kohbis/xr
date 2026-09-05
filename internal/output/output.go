package output

import (
	"encoding/json"
	"fmt"
	"io"
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

// RepoHeader returns a repository header block, including its leading and
// trailing newlines, for callers writing to their own writer.
func RepoHeader(name string) string {
	return fmt.Sprintf("\n%s%s=== %s ===%s\n", c(colorBold), c(colorCyan), name, c(colorReset))
}

func PrintRepoHeader(name string) {
	fmt.Print(RepoHeader(name))
}

// Dim returns msg rendered in the dimmed style used for de-emphasized lines.
func Dim(msg string) string {
	return c(colorDim) + msg + c(colorReset)
}

// Red returns msg rendered in the style used for failures.
func Red(msg string) string {
	return c(colorRed) + msg + c(colorReset)
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

// SyncPrinter renders sync progress to a writer. Concurrent sync workers each
// render into their own buffer, so the combined output can be flushed in
// repository order and read exactly like a sequential run.
type SyncPrinter struct {
	w    io.Writer
	errW io.Writer

	events []SyncEvent
}

// SyncEvent is one progress line rendered by a SyncPrinter, kept so callers
// can report the same steps in machine-readable form.
type SyncEvent struct {
	Kind    string `json:"kind"` // action, ok, skip, fail
	Message string `json:"message"`
}

const (
	SyncEventAction = "action"
	SyncEventOK     = "ok"
	SyncEventSkip   = "skip"
	SyncEventFail   = "fail"
)

// Events returns the progress lines rendered so far, in order.
func (p *SyncPrinter) Events() []SyncEvent {
	return p.events
}

func (p *SyncPrinter) record(kind, msg string) {
	p.events = append(p.events, SyncEvent{Kind: kind, Message: msg})
}

// NewSyncPrinter returns a printer whose progress lines go to out. Subprocess
// output is streamed through Writer and ErrWriter so each stream keeps its
// identity.
func NewSyncPrinter(out, err io.Writer) *SyncPrinter {
	return &SyncPrinter{w: out, errW: err}
}

// Writer returns the stdout writer, for subprocesses that stream their own
// output (git clone progress, for example).
func (p *SyncPrinter) Writer() io.Writer {
	return p.w
}

// ErrWriter returns the stderr writer for subprocess output.
func (p *SyncPrinter) ErrWriter() io.Writer {
	return p.errW
}

// Header prints a repo header for sync operations.
func (p *SyncPrinter) Header(name, repoType string) {
	_, _ = fmt.Fprintf(p.w, "\n%s%s[%s]%s %s%s%s\n", c(colorBold), c(colorCyan), repoType, c(colorReset), c(colorBold), name, c(colorReset))
}

// Skip prints a skip message for repos that don't need syncing.
func (p *SyncPrinter) Skip(reason string) {
	p.record(SyncEventSkip, reason)
	_, _ = fmt.Fprintf(p.w, "  %s⊘ %s%s\n", c(colorDim), reason, c(colorReset))
}

// OK prints a success message for a sync step.
func (p *SyncPrinter) OK(msg string) {
	p.record(SyncEventOK, msg)
	_, _ = fmt.Fprintf(p.w, "  %s✓ %s%s\n", c(colorGreen), msg, c(colorReset))
}

// Action prints an action being performed.
func (p *SyncPrinter) Action(msg string) {
	p.record(SyncEventAction, msg)
	_, _ = fmt.Fprintf(p.w, "  %s→%s %s\n", c(colorBlue), c(colorReset), msg)
}

// Fail prints a failure message for a sync step.
func (p *SyncPrinter) Fail(msg string) {
	p.record(SyncEventFail, msg)
	_, _ = fmt.Fprintf(p.w, "  %s✗ %s%s\n", c(colorRed), msg, c(colorReset))
}

// PrintActionSummary prints the final summary of a multi-repository operation.
// changedLabel names what happened to the changed repos (e.g. "created").
func PrintActionSummary(changedLabel string, changed, skipped, failed int) {
	parts := []string{}
	if changed > 0 {
		parts = append(parts, fmt.Sprintf("%s%d %s%s", c(colorGreen), changed, changedLabel, c(colorReset)))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%s%d skipped%s", c(colorDim), skipped, c(colorReset)))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%s%d failed%s", c(colorRed), failed, c(colorReset)))
	}
	if len(parts) == 0 {
		parts = append(parts, "nothing to do")
	}
	fmt.Printf("\nDone: %s\n", strings.Join(parts, ", "))
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
