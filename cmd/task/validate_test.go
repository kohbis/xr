package task

import (
	"testing"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/task"
)

func TestValidateTaskFile_OK(t *testing.T) {
	tf := &task.File{
		Version: 1,
		Tasks: []task.Task{
			{
				ID:        "implement-authentication",
				CreatedAt: "2026-04-29",
				Repos:     []string{"api"},
				Steps: []task.Step{
					{ID: "search", Type: task.StepTypeRun, Run: "xr search foo"},
					{ID: "impl", Type: task.StepTypeAgent, Instruction: "do it"},
				},
			},
		},
	}
	repos := cfgRepoNames(&config.Config{Repositories: []config.Repository{{Name: "api"}}})
	if err := validateTaskFile(tf, repos); err != nil {
		t.Fatalf("validateTaskFile() error = %v", err)
	}
}

func TestValidateTaskFile_RejectsAgentWithRun(t *testing.T) {
	tf := &task.File{
		Version: 1,
		Tasks: []task.Task{
			{
				ID: "a",
				Steps: []task.Step{
					{ID: "s1", Type: task.StepTypeAgent, Instruction: "x", Run: "echo nope"},
				},
			},
		},
	}
	repos := cfgRepoNames(&config.Config{})
	if err := validateTaskFile(tf, repos); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestValidateTaskFile_RequiresStepID(t *testing.T) {
	tf := &task.File{
		Version: 1,
		Tasks: []task.Task{
			{
				ID: "a",
				Steps: []task.Step{
					{Type: task.StepTypeRun, Run: "echo hi"},
				},
			},
		},
	}
	repos := cfgRepoNames(&config.Config{})
	if err := validateTaskFile(tf, repos); err == nil {
		t.Fatal("expected error, got nil")
	}
}
