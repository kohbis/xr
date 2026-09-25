// Package pathsafe checks that a derived path stays inside the directory it is
// meant to live in.
//
// Symlinks along the way are resolved, so a path cannot leave through a
// symlinked parent. The final component is not: xr puts a symlink there for
// every symlink repository, and a caller acts on the link, not its target.
package pathsafe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Inside reports whether path is contained within dir. dir itself is not
// "inside" dir: a caller about to create or delete path would otherwise be
// handed the directory it is supposed to be confined to.
func Inside(dir, path string) error {
	absDir, err := resolveExisting(dir)
	if err != nil {
		return fmt.Errorf("resolving %q: %w", dir, err)
	}
	parent, err := resolveExisting(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("resolving %q: %w", filepath.Dir(path), err)
	}
	absPath := filepath.Join(parent, filepath.Base(path))

	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return fmt.Errorf("relating %q to %q: %w", path, dir, err)
	}
	// Whole elements, not a string prefix, which would reject a sibling named "..foo".
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path %q escapes %q", path, dir)
	}
	return nil
}

// resolveExisting resolves symlinks in the deepest part of path that exists,
// leaving the components below it as given.
func resolveExisting(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var missing []string
	for current := abs; ; {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Join(append([]string{resolved}, missing...)...), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return abs, nil
		}
		missing = append([]string{filepath.Base(current)}, missing...)
		current = parent
	}
}
