package work

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Repo represents a repository reference in a work plan.
// Branch is optional. When set, commands like `xr repo sync --work <name>` may
// switch that repository to the specified branch.
type Repo struct {
	Name   string `yaml:"name"`
	Branch string `yaml:"branch,omitempty"`
}

func (r *Repo) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	switch value.Kind {
	case yaml.MappingNode:
		type alias Repo
		var a alias
		if err := value.Decode(&a); err != nil {
			return err
		}
		*r = Repo(a)
		if r.Name == "" {
			return fmt.Errorf("repo entry: name is required")
		}
		return nil
	default:
		return fmt.Errorf("repo entry must be a mapping with name/branch")
	}
}

// File represents .xr/work/<name>.yaml (schema v1).
type File struct {
	Name  string `yaml:"name"`
	Repos []Repo `yaml:"repos"`
}

func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading work file: %w", err)
	}
	var f File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parsing work file: %w", err)
	}
	if f.Repos == nil {
		f.Repos = []Repo{}
	}
	return &f, nil
}
