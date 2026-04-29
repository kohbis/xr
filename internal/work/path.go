package work

import (
	"fmt"
	"path/filepath"
	"strings"
)

// NormalizeName strips optional YAML extensions from a work plan name.
func NormalizeName(name string) string {
	name = strings.TrimSuffix(name, ".yaml")
	name = strings.TrimSuffix(name, ".yml")
	return name
}

func ValidateName(name string) error {
	n := NormalizeName(name)
	if n == "" {
		return fmt.Errorf("work name is required")
	}
	if strings.Contains(n, "/") || strings.Contains(n, "\\") {
		return fmt.Errorf("invalid work name %q", name)
	}
	if n == "." || n == ".." {
		return fmt.Errorf("invalid work name %q", name)
	}
	return nil
}

// Dir returns the .xr/work directory path under the given root.
func Dir(root string) string {
	return filepath.Join(root, ".xr", "work")
}

// FilePath returns the absolute work file path for the given name under the given root.
func FilePath(root, name string) string {
	return filepath.Join(Dir(root), NormalizeName(name)+".yaml")
}

func SafeFilePath(root, name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	return FilePath(root, name), nil
}

