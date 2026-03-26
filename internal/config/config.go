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
	RepoTypeClone   RepoType = "clone"
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

	return normalize(&cfg)
}

func (r *Repository) IsSymlink() bool {
	return r.Type == RepoTypeSymlink
}

func (r *Repository) IsClone() bool {
	return r.Type == RepoTypeClone
}

func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Reload marshals and re-parses a config to apply type inference and validation.
func Reload(cfg *Config) (*Config, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshaling config: %w", err)
	}
	var reloaded Config
	if err := yaml.Unmarshal(data, &reloaded); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return normalize(&reloaded)
}

// normalize applies defaults, type inference, and validation to a config.
func normalize(cfg *Config) (*Config, error) {
	if cfg.Workspace == "" {
		cfg.Workspace = "./repos"
	}

	for i, repo := range cfg.Repositories {
		if repo.Type == "" {
			if len(repo.Source) > 0 && (repo.Source[0] == '/' || repo.Source[0] == '~') {
				cfg.Repositories[i].Type = RepoTypeSymlink
			} else {
				cfg.Repositories[i].Type = RepoTypeGit
			}
		}
		switch cfg.Repositories[i].Type {
		case RepoTypeGit, RepoTypeSymlink, RepoTypeClone:
			// valid
		default:
			return nil, fmt.Errorf("repository %q: unknown type %q", repo.Name, repo.Type)
		}
		if repo.Path == "" {
			cfg.Repositories[i].Path = repo.Name
		}
	}

	return cfg, nil
}
