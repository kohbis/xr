package task

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/exitcode"
	"github.com/kohbis/xr/internal/task"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate xr-tasks.yaml",
	Long:  "Validate the task definition file (schema, ids, repo references, and step constraints).",
	RunE: func(cmd *cobra.Command, args []string) error {
		tf, _, err := loadTasksFile(cmd)
		if err != nil {
			return exitcode.Errorf(2, "%v", err)
		}
		cfg, _, err := loadRepoConfig(cmd)
		if err != nil {
			return exitcode.Errorf(2, "%v", err)
		}

		if err := validateTaskFile(tf, cfgRepoNames(cfg)); err != nil {
			return exitcode.Errorf(2, "%v", err)
		}

		fmt.Println("OK")
		return nil
	},
}

func init() {
}

func cfgRepoNames(cfg *config.Config) map[string]struct{} {
	out := make(map[string]struct{}, len(cfg.Repositories))
	for _, r := range cfg.Repositories {
		out[r.Name] = struct{}{}
	}
	return out
}

var (
	taskIDRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

func validateTaskFile(tf *task.File, repoNames map[string]struct{}) error {
	if tf.Version != 1 {
		return fmt.Errorf("unsupported tasks schema version: %d", tf.Version)
	}
	seenTask := make(map[string]struct{}, len(tf.Tasks))
	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		if t.ID == "" {
			return fmt.Errorf("tasks[%d].id is required", i)
		}
		if !taskIDRe.MatchString(t.ID) {
			return fmt.Errorf("task %q: id must be kebab-case (got %q)", t.ID, t.ID)
		}
		if _, ok := seenTask[t.ID]; ok {
			return fmt.Errorf("duplicate task id: %q", t.ID)
		}
		seenTask[t.ID] = struct{}{}

		if t.CreatedAt != "" {
			if _, err := task.ParseCreatedAt(t.CreatedAt); err != nil {
				return fmt.Errorf("task %q: createdAt must be YYYY-MM-DD: %w", t.ID, err)
			}
		}

		for _, r := range t.Repos {
			if _, ok := repoNames[r]; !ok {
				return fmt.Errorf("task %q: unknown repo %q", t.ID, r)
			}
		}

		seenStep := make(map[string]struct{}, len(t.Steps))
		for si := range t.Steps {
			s := &t.Steps[si]
			if s.ID == "" {
				return fmt.Errorf("task %q: steps[%d].id is required", t.ID, si)
			}
			if !taskIDRe.MatchString(s.ID) {
				return fmt.Errorf("task %q: step id must be kebab-case (got %q)", t.ID, s.ID)
			}
			if _, ok := seenStep[s.ID]; ok {
				return fmt.Errorf("task %q: duplicate step id: %q", t.ID, s.ID)
			}
			seenStep[s.ID] = struct{}{}

			switch s.Type {
			case task.StepTypeRun:
				if strings.TrimSpace(s.Run) == "" {
					return fmt.Errorf("task %q: step %q: run is required for type=run", t.ID, s.ID)
				}
				if s.Repo != "" && len(s.Repos) > 0 {
					return fmt.Errorf("task %q: step %q: use only one of repo or repos", t.ID, s.ID)
				}
				if s.Repo != "" {
					if _, ok := repoNames[s.Repo]; !ok {
						return fmt.Errorf("task %q: step %q: unknown repo %q", t.ID, s.ID, s.Repo)
					}
				}
				for _, r := range s.Repos {
					if _, ok := repoNames[r]; !ok {
						return fmt.Errorf("task %q: step %q: unknown repo %q", t.ID, s.ID, r)
					}
				}
			case task.StepTypeAgent:
				if strings.TrimSpace(s.Instruction) == "" {
					return fmt.Errorf("task %q: step %q: instruction is required for type=agent", t.ID, s.ID)
				}
				if strings.TrimSpace(s.Run) != "" {
					return fmt.Errorf("task %q: step %q: run must be empty for type=agent", t.ID, s.ID)
				}
				if s.Repo != "" || len(s.Repos) > 0 {
					return fmt.Errorf("task %q: step %q: repo targeting is not allowed for type=agent", t.ID, s.ID)
				}
			default:
				return fmt.Errorf("task %q: step %q: unknown type %q", t.ID, s.ID, s.Type)
			}
		}
	}
	return nil
}
