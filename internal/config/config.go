package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type RepoType string

const (
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
	Worktrees    string       `yaml:"worktrees,omitempty"`
	Repositories []Repository `yaml:"repositories"`

	// Path is the absolute path of the file the config was loaded from. The
	// workspace and worktrees directories are resolved relative to its
	// directory, so a command behaves the same from any working directory when
	// --config points at another workspace. Empty when the config was built in
	// memory, in which case the current directory is used.
	Path string `yaml:"-"`
}

// DefaultPath is the config file used when --config is not given.
const DefaultPath = "repos.yaml"

// CommandPath returns the config file selected by the global --config flag of
// cmd's root command. Without the flag it is the nearest repos.yaml at or above
// the working directory, so commands work from inside a repository of the
// workspace; when there is none, it falls back to DefaultPath in the working
// directory, which is where a config would be created.
func CommandPath(cmd *cobra.Command) string {
	if cmd != nil {
		if f := cmd.Root().PersistentFlags().Lookup("config"); f != nil && f.Value.String() != "" {
			return f.Value.String()
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return DefaultPath
	}
	if found := FindPath(wd); found != "" {
		return found
	}
	return DefaultPath
}

// FindPath returns the nearest repos.yaml at or above startDir, or "" when no
// parent directory up to the filesystem root holds one. The walk deliberately
// does not stop at a repository boundary: the workspace config sits above the
// repositories it manages, so it is found from inside one of them.
func FindPath(startDir string) string {
	dir := startDir
	for {
		candidate := filepath.Join(dir, DefaultPath)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// LoadCommand loads the config selected by the global --config flag.
func LoadCommand(cmd *cobra.Command) (*Config, error) {
	return Load(CommandPath(cmd))
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

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving config path: %w", err)
	}
	cfg.Path = abs

	return normalize(&cfg)
}

// Root returns the directory that workspace-relative paths are resolved
// against: the directory containing the config file, or the current
// directory for a config built in memory.
func (c *Config) Root() string {
	if c.Path == "" {
		return "."
	}
	return filepath.Dir(c.Path)
}

// WorkspaceDir returns the absolute path of the directory holding the
// repositories.
func (c *Config) WorkspaceDir() (string, error) {
	return filepath.Abs(filepath.Join(c.Root(), c.Workspace))
}

// WorktreesDir returns the absolute path of the directory holding worktrees.
func (c *Config) WorktreesDir() (string, error) {
	return filepath.Abs(filepath.Join(c.Root(), c.Worktrees))
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
	reloaded.Path = cfg.Path
	return normalize(&reloaded)
}

// normalize applies defaults, type inference, and validation to a config.
func normalize(cfg *Config) (*Config, error) {
	if cfg.Workspace == "" {
		cfg.Workspace = "./repos"
	}
	if cfg.Worktrees == "" {
		cfg.Worktrees = "./worktrees"
	}

	for i, repo := range cfg.Repositories {
		if repo.Type == "" {
			if len(repo.Source) > 0 && (repo.Source[0] == '/' || repo.Source[0] == '~') {
				cfg.Repositories[i].Type = RepoTypeSymlink
			} else {
				cfg.Repositories[i].Type = RepoTypeClone
			}
		}
		switch cfg.Repositories[i].Type {
		case RepoTypeSymlink, RepoTypeClone:
			// valid
		case "git":
			return nil, fmt.Errorf("repository %q: type %q is no longer supported (use %q)", repo.Name, repo.Type, RepoTypeClone)
		default:
			return nil, fmt.Errorf("repository %q: unknown type %q", repo.Name, repo.Type)
		}
		if repo.Path == "" {
			cfg.Repositories[i].Path = repo.Name
		}
	}

	return cfg, nil
}
