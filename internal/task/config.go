package task

import (
	"crypto/sha256"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type StepType string

const (
	StepTypeRun   StepType = "run"
	StepTypeAgent StepType = "agent"
)

// File represents xr-tasks.yaml (schema v1).
type File struct {
	Version int    `yaml:"version"`
	Tasks   []Task `yaml:"tasks"`
}

type Task struct {
	ID          string   `yaml:"id"`
	CreatedAt   string   `yaml:"createdAt,omitempty"`
	Title       string   `yaml:"title,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Status      string   `yaml:"status,omitempty"`
	Repos       []string `yaml:"repos,omitempty"`

	Goals              []string  `yaml:"goals,omitempty"`
	AcceptanceCriteria []string  `yaml:"acceptanceCriteria,omitempty"`
	Subtasks           []Subtask `yaml:"subtasks,omitempty"`

	Steps []Step `yaml:"steps,omitempty"`
}

type Subtask struct {
	ID     string `yaml:"id"`
	Title  string `yaml:"title,omitempty"`
	Status string `yaml:"status,omitempty"`
}

type Step struct {
	ID   string   `yaml:"id"`
	Type StepType `yaml:"type"`

	// run step
	Run     string   `yaml:"run,omitempty"`
	Repo    string   `yaml:"repo,omitempty"`
	Repos   []string `yaml:"repos,omitempty"`
	WorkDir string   `yaml:"workingDir,omitempty"`

	ContinueOnError bool `yaml:"continueOnError,omitempty"`
	TimeoutSeconds  int  `yaml:"timeoutSeconds,omitempty"`

	// agent step
	Instruction        string   `yaml:"instruction,omitempty"`
	AcceptanceCriteria []string `yaml:"acceptanceCriteria,omitempty"`
}

func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading tasks file: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) (*File, error) {
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing tasks file: %w", err)
	}
	if f.Version == 0 {
		// default to schema v1 for early adopters
		f.Version = 1
	}
	return &f, nil
}

func HashFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:]), nil
}

func ParseCreatedAt(s string) (time.Time, error) {
	// Phase A: date only, YYYY-MM-DD
	return time.Parse("2006-01-02", s)
}
