// Package pathsafe checks that a derived path stays inside the directory it is
// meant to live in.
//
// Repository and worktree paths come from repos.yaml and from branch names, so
// they are attacker-adjacent input that ends up in os.RemoveAll and git
// worktree add. The containment check is the boundary that keeps those writes
// inside the workspace, and it lives here so there is one implementation of it
// rather than one per package.
package pathsafe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Inside reports whether path is contained within dir, as an error describing
// the escape when it is not. dir itself is not "inside" dir: a caller that is
// about to create or delete path would otherwise be handed the directory it is
// supposed to be confined to.
func Inside(dir, path string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return err
	}
	// Compare whole path elements: a plain "*.." prefix test would also reject a
	// sibling legitimately named "..foo".
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path %q escapes %q", path, dir)
	}
	return nil
}
