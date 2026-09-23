// Package pathsafe checks that a derived path stays inside the directory it is
// meant to live in.
//
// The check is lexical: it compares the two paths as text and never touches the
// filesystem. It catches a path that spells its way out with "..", not one that
// leaves through a symlinked parent — if repos/link points outside, repos/link/x
// is still "inside" repos. A caller that must not follow a symlink out has to
// resolve the path itself, or work through os.OpenRoot.
package pathsafe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Inside reports whether path is lexically contained within dir. dir itself is
// not "inside" dir: a caller about to create or delete path would otherwise be
// handed the directory it is supposed to be confined to.
func Inside(dir, path string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving %q: %w", dir, err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving %q: %w", path, err)
	}
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
