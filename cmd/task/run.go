package task

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/exitcode"
	"github.com/kohbis/xr/internal/task"
	"github.com/spf13/cobra"
)

const (
	exitRunFailed   = 1
	exitValidate    = 2
	exitLock        = 3
	exitInterrupted = 130
)

var (
	runJSON           bool
	runReportPath     string
	runReportDir      string
	runSkipAgentGates bool
	runFromStep       string
	runOnlySteps      []string
	runResume         bool
	runForce          bool
	runRequireClean   bool
)

var runCmd = &cobra.Command{
	Use:   "run <id>",
	Short: "Run a task's run-steps and write a report",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		tf, tasksPath, err := loadTasksFile(cmd)
		if err != nil {
			return exitcode.Errorf(exitValidate, "%v", err)
		}
		cfg, cfgPath, err := loadRepoConfig(cmd)
		if err != nil {
			return exitcode.Errorf(exitValidate, "%v", err)
		}

		workspaceRoot, err := resolveWorkspaceRoot(cmd)
		if err != nil {
			return err
		}

		// Validate before any side effects.
		if err := validateTaskFile(tf, cfgRepoNames(cfg)); err != nil {
			return exitcode.Errorf(exitValidate, "%v", err)
		}

		t, err := findTaskByID(tf, taskID)
		if err != nil {
			return exitcode.Errorf(exitValidate, "%v", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		lockFile, unlock, err := lockWorkspace(workspaceRoot)
		if err != nil {
			return exitcode.Errorf(exitLock, "%v", err)
		}
		defer unlock()

		taskHash, _ := fileHash(tasksPath)
		reposHash, _ := fileHash(cfgPath)

		resolvedRepos := resolveTaskReposSnapshot(cfg, workspaceRoot, t)

		report := newReport(taskID, taskHash, reposHash, resolvedRepos)
		reportPath, err := computeReportPath(workspaceRoot, taskID)
		if err != nil {
			return err
		}
		if runReportDir != "" {
			reportPath = filepath.Join(runReportDir, filepath.Base(reportPath))
		}
		if runReportPath != "" {
			reportPath = runReportPath
		}
		report.ReportPath = reportPath
		report.WorkspaceLockPath = lockFile

		if runResume {
			latest, err := findLatestReport(workspaceRoot, taskID)
			if err == nil {
				if !runForce && (latest.Inputs.TasksFileHash != report.Inputs.TasksFileHash || latest.Inputs.ReposFileHash != report.Inputs.ReposFileHash) {
					return exitcode.Errorf(exitValidate, "cannot resume: inputs changed (use --force)")
				}
				report.ResumeFrom = latest.ReportPath
				report.ResumeSkippedStepIDs = succeededRunSteps(latest)
			}
		}

		onlySet := make(map[string]struct{})
		for _, s := range runOnlySteps {
			if s != "" {
				onlySet[s] = struct{}{}
			}
		}
		startFrom := runFromStep == ""

		started := time.Now().UTC()
		report.StartedAt = started.Format(time.RFC3339Nano)
		report.Status = "running"
		_ = writeReportAtomic(reportPath, report) // best-effort

		var runErr error
		for _, step := range t.Steps {
			if !startFrom {
				if step.ID != runFromStep {
					continue
				}
				startFrom = true
			}

			if step.Type == task.StepTypeAgent {
				report.Steps = append(report.Steps, ReportStep{
					StepID:      step.ID,
					Type:        string(step.Type),
					Status:      "pending",
					Instruction: step.Instruction,
				})
				_ = writeReportAtomic(reportPath, report)
				if !runSkipAgentGates {
					report.Status = "blocked"
					report.EndedAt = time.Now().UTC().Format(time.RFC3339Nano)
					report.DurationMs = msSince(started)
					_ = writeReportAtomic(reportPath, report)
					return exitcode.Errorf(exitRunFailed, "blocked on agent step %q (use --skip-agent-gates to ignore)", step.ID)
				}
				continue
			}

			if len(onlySet) > 0 {
				if _, ok := onlySet[step.ID]; !ok {
					continue
				}
			}
			if runResume {
				if contains(report.ResumeSkippedStepIDs, step.ID) {
					report.Steps = append(report.Steps, ReportStep{
						StepID: step.ID,
						Type:   string(step.Type),
						Status: "skipped",
					})
					_ = writeReportAtomic(reportPath, report)
					continue
				}
			}

			targetRepos, err := resolveStepRepos(cfg, workspaceRoot, step, resolvedRepos)
			if err != nil {
				runErr = err
				break
			}
			if runRequireClean {
				if err := requireCleanRepos(targetRepos); err != nil {
					runErr = err
					break
				}
			}

			stepTimeout := step.TimeoutSeconds
			if stepTimeout <= 0 {
				stepTimeout = 600
			}

			// Run once per target repo; if no target repos, run in workspace root.
			if len(targetRepos) == 0 {
				rs, err := runOne(ctx, reportPath, step, RunTarget{CWD: workspaceRoot}, time.Duration(stepTimeout)*time.Second)
				report.Steps = append(report.Steps, rs)
				_ = writeReportAtomic(reportPath, report)
				if err != nil {
					runErr = err
					break
				}
			} else {
				for _, rt := range targetRepos {
					rs, err := runOne(ctx, reportPath, step, rt, time.Duration(stepTimeout)*time.Second)
					report.Steps = append(report.Steps, rs)
					_ = writeReportAtomic(reportPath, report)
					if err != nil {
						runErr = err
						if step.ContinueOnError {
							continue
						}
						break
					}
				}
				if runErr != nil && !step.ContinueOnError {
					break
				}
			}
		}

		switch {
		case errors.Is(runErr, context.Canceled):
			report.Status = "interrupted"
			report.ExitCode = exitInterrupted
		case runErr != nil:
			report.Status = "failed"
			report.ExitCode = exitRunFailed
		default:
			report.Status = "succeeded"
			report.ExitCode = 0
		}

		report.EndedAt = time.Now().UTC().Format(time.RFC3339Nano)
		report.DurationMs = msSince(started)
		_ = writeReportAtomic(reportPath, report)

		if runJSON {
			_ = json.NewEncoder(os.Stdout).Encode(report)
		}

		if runErr != nil {
			if errors.Is(runErr, context.Canceled) {
				return exitcode.Errorf(exitInterrupted, "interrupted")
			}
			return exitcode.Errorf(exitRunFailed, "%v", runErr)
		}
		return nil
	},
}

func init() {
	runCmd.Flags().BoolVar(&runJSON, "json", false, "output final report JSON to stdout")
	runCmd.Flags().StringVar(&runReportPath, "report", "", "write report JSON to the given path")
	runCmd.Flags().StringVar(&runReportDir, "report-dir", "", "write report JSON under the given directory")
	runCmd.Flags().BoolVar(&runSkipAgentGates, "skip-agent-gates", false, "ignore agent steps and continue executing run steps")
	runCmd.Flags().StringVar(&runFromStep, "from", "", "start executing from the given step id")
	runCmd.Flags().StringArrayVar(&runOnlySteps, "only", nil, "execute only the given step id(s)")
	runCmd.Flags().BoolVar(&runResume, "resume", false, "resume from the latest report for this task")
	runCmd.Flags().BoolVar(&runForce, "force", false, "force resume even if inputs changed")
	runCmd.Flags().BoolVar(&runRequireClean, "require-clean", false, "require a clean git working tree in targeted repos")
}

type RunTarget struct {
	RepoName string `json:"repoName,omitempty"`
	RepoPath string `json:"repoPath,omitempty"`
	CWD      string `json:"cwd"`
}

type Report struct {
	SchemaVersion int    `json:"schemaVersion"`
	TaskID        string `json:"taskId"`
	Status        string `json:"status"`
	ExitCode      int    `json:"exitCode"`

	StartedAt  string `json:"startedAt"`
	EndedAt    string `json:"endedAt,omitempty"`
	DurationMs int64  `json:"durationMs"`

	XRVersion string `json:"xrVersion"`

	ReportPath        string `json:"reportPath"`
	WorkspaceLockPath string `json:"workspaceLockPath"`

	Inputs ReportInputs `json:"inputs"`

	Steps []ReportStep `json:"steps"`

	ResumeFrom           string   `json:"resumeFrom,omitempty"`
	ResumeSkippedStepIDs []string `json:"resumeSkippedStepIds,omitempty"`
}

type ReportInputs struct {
	TasksFileHash string      `json:"tasksFileHash"`
	ReposFileHash string      `json:"reposFileHash"`
	ResolvedRepos []RunTarget `json:"resolvedRepos"`
}

type ReportStep struct {
	StepID string `json:"stepId"`
	Type   string `json:"type"`

	Status     string `json:"status"`
	StartedAt  string `json:"startedAt,omitempty"`
	EndedAt    string `json:"endedAt,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`

	CWD     string `json:"cwd,omitempty"`
	Command string `json:"command,omitempty"`
	Repo    string `json:"repo,omitempty"`

	ExitCode int `json:"exitCode,omitempty"`

	StdoutSnippet string `json:"stdoutSnippet,omitempty"`
	StderrSnippet string `json:"stderrSnippet,omitempty"`

	Instruction string `json:"instruction,omitempty"`
}

func newReport(taskID, tasksHash, reposHash string, resolved []RunTarget) *Report {
	return &Report{
		SchemaVersion: 1,
		TaskID:        taskID,
		XRVersion:     "dev",
		Inputs: ReportInputs{
			TasksFileHash: tasksHash,
			ReposFileHash: reposHash,
			ResolvedRepos: resolved,
		},
	}
}

func computeReportPath(workspaceRoot, taskID string) (string, error) {
	dir := filepath.Join(workspaceRoot, ".xr", "reports", taskID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	name := time.Now().UTC().Format("2006-01-02T150405.000000000Z") + ".json"
	return filepath.Join(dir, name), nil
}

func writeReportAtomic(path string, r *Report) error {
	tmp := path + ".tmp"
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func msSince(t time.Time) int64 {
	return time.Since(t).Milliseconds()
}

func fileHash(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:]), nil
}

func lockWorkspace(workspaceRoot string) (lockPath string, unlock func(), err error) {
	lockDir := filepath.Join(workspaceRoot, ".xr", "locks")
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return "", nil, err
	}
	lockPath = filepath.Join(lockDir, "workspace.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return "", nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return "", nil, fmt.Errorf("workspace is locked: %w", err)
	}
	return lockPath, func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

func resolveTaskReposSnapshot(cfg *config.Config, workspaceRoot string, t *task.Task) []RunTarget {
	byName := map[string]config.Repository{}
	for _, r := range cfg.Repositories {
		byName[r.Name] = r
	}
	var out []RunTarget
	for _, name := range t.Repos {
		if repo, ok := byName[name]; ok {
			dir, _ := resolveRepoDir(workspaceRoot, cfg, repo)
			out = append(out, RunTarget{RepoName: name, RepoPath: repo.Path, CWD: dir})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RepoName < out[j].RepoName })
	return out
}

func resolveStepRepos(cfg *config.Config, workspaceRoot string, step task.Step, snapshot []RunTarget) ([]RunTarget, error) {
	if step.Repo == "" && len(step.Repos) == 0 {
		return nil, nil
	}

	byName := map[string]config.Repository{}
	for _, r := range cfg.Repositories {
		byName[r.Name] = r
	}

	allowed := map[string]RunTarget{}
	for _, rt := range snapshot {
		allowed[rt.RepoName] = rt
	}

	var names []string
	if step.Repo != "" {
		names = []string{step.Repo}
	} else {
		names = append(names, step.Repos...)
	}

	var out []RunTarget
	for _, name := range names {
		if len(snapshot) > 0 {
			if rt, ok := allowed[name]; ok {
				out = append(out, rt)
				continue
			}
			return nil, fmt.Errorf("step %q targets repo %q not in task repos snapshot", step.ID, name)
		}
		repo, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("unknown repo %q", name)
		}
		dir, err := resolveRepoDir(workspaceRoot, cfg, repo)
		if err != nil {
			return nil, err
		}
		out = append(out, RunTarget{RepoName: name, RepoPath: repo.Path, CWD: dir})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].RepoName < out[j].RepoName })
	return out, nil
}

func requireCleanRepos(targets []RunTarget) error {
	for _, t := range targets {
		cmd := exec.Command("git", "status", "--porcelain")
		cmd.Dir = t.CWD
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("%s: git status: %w", t.RepoName, err)
		}
		if len(bytes.TrimSpace(out)) > 0 {
			return fmt.Errorf("%s: dirty working tree", t.RepoName)
		}
	}
	return nil
}

func runOne(ctx context.Context, reportPath string, step task.Step, target RunTarget, timeout time.Duration) (ReportStep, error) {
	started := time.Now().UTC()
	rs := ReportStep{
		StepID:    step.ID,
		Type:      string(step.Type),
		Status:    "running",
		StartedAt: started.Format(time.RFC3339Nano),
		CWD:       target.CWD,
		Command:   step.Run,
		Repo:      target.RepoName,
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	command := exec.CommandContext(ctx, "bash", "-lc", step.Run)
	command.Dir = target.CWD
	command.Stdin = nil

	var stdoutBuf, stderrBuf limitedBuffer
	command.Stdout = &stdoutBuf
	command.Stderr = &stderrBuf

	err := command.Run()
	ended := time.Now().UTC()
	rs.EndedAt = ended.Format(time.RFC3339Nano)
	rs.DurationMs = ended.Sub(started).Milliseconds()
	rs.StdoutSnippet = stdoutBuf.String()
	rs.StderrSnippet = stderrBuf.String()

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			rs.Status = "timed_out"
			return rs, fmt.Errorf("step %q timed out", step.ID)
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			rs.Status = "interrupted"
			return rs, context.Canceled
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			rs.ExitCode = ee.ExitCode()
		}
		rs.Status = "failed"
		return rs, fmt.Errorf("step %q failed: %w", step.ID, err)
	}

	rs.Status = "succeeded"
	rs.ExitCode = 0
	_ = reportPath
	return rs, nil
}

type limitedBuffer struct {
	buf bytes.Buffer
	max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.max == 0 {
		b.max = 16 * 1024
	}
	if b.buf.Len()+len(p) > b.max {
		remain := b.max - b.buf.Len()
		if remain > 0 {
			_, _ = b.buf.Write(p[:remain])
		}
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *limitedBuffer) String() string { return b.buf.String() }

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func findLatestReport(workspaceRoot, taskID string) (*Report, error) {
	dir := filepath.Join(workspaceRoot, ".xr", "reports", taskID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no reports found")
	}
	sort.Strings(names)
	latest := filepath.Join(dir, names[len(names)-1])
	b, err := os.ReadFile(latest)
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	if r.ReportPath == "" {
		r.ReportPath = latest
	}
	return &r, nil
}

func succeededRunSteps(r *Report) []string {
	var out []string
	for _, s := range r.Steps {
		if s.Type == "run" && s.Status == "succeeded" {
			out = append(out, s.StepID)
		}
	}
	return out
}
