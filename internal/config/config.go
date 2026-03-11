package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type RepoType string

const (
	RepoTypeGit     RepoType = "git"
	RepoTypeSymlink RepoType = "symlink"
)

type Repository struct {
	Name   string   `yaml:"name"`
	Source string   `yaml:"source"`
	Branch string   `yaml:"branch,omitempty"`
	Path   string   `yaml:"path"`
	Type   RepoType `yaml:"type,omitempty"`
}

type Config struct {
	Workspace    string       `yaml:"workspace"`
	Repositories []Repository `yaml:"repositories"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if cfg.Workspace == "" {
		cfg.Workspace = "./repos"
	}

	for i, repo := range cfg.Repositories {
		if repo.Type == "" {
			cfg.Repositories[i].Type = RepoTypeGit
		}
		if repo.Path == "" {
			cfg.Repositories[i].Path = repo.Name
		}
	}

	return &cfg, nil
}

func (r *Repository) IsSymlink() bool {
	if r.Type == RepoTypeSymlink {
		return true
	}
	// Heuristic: local paths (absolute or home-relative) are symlinks
	if len(r.Source) > 0 && (r.Source[0] == '/' || r.Source[0] == '~') {
		return true
	}
	return false
}

func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

