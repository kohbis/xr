package task

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/task"
	"github.com/spf13/cobra"
)

const defaultTasksFile = "xr-tasks.yaml"

func loadRepoConfig(cmd *cobra.Command) (*config.Config, string, error) {
	cfgPath := cmd.Root().PersistentFlags().Lookup("config").Value.String()
	if cfgPath == "" {
		cfgPath = "repos.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, "", err
	}
	return cfg, cfgPath, nil
}

func resolveWorkspaceRoot(cmd *cobra.Command) (string, error) {
	// For Phase A, we treat the current working directory as the workspace root.
	// (This matches existing commands that interpret relative paths from cwd.)
	return os.Getwd()
}

func tasksFilePath(cmd *cobra.Command) string {
	if v, err := cmd.Flags().GetString("tasks"); err == nil && v != "" {
		return v
	}
	return defaultTasksFile
}

func loadTasksFile(cmd *cobra.Command) (*task.File, string, error) {
	p := tasksFilePath(cmd)
	tf, err := task.Load(p)
	if err != nil {
		return nil, p, err
	}
	return tf, p, nil
}

func resolveRepoDir(workspaceRoot string, cfg *config.Config, repo config.Repository) (string, error) {
	ws, err := filepath.Abs(cfg.Workspace)
	if err != nil {
		return "", err
	}
	_ = workspaceRoot // reserved for future: make workspace relative to config dir
	return filepath.Join(ws, repo.Path), nil
}

func findTaskByID(tf *task.File, id string) (*task.Task, error) {
	for i := range tf.Tasks {
		if tf.Tasks[i].ID == id {
			return &tf.Tasks[i], nil
		}
	}
	return nil, fmt.Errorf("task %q not found", id)
}
