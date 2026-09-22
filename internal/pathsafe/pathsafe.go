// Package pathsafe checks that a derived path stays inside the directory it is
// meant to live in.
//
// Repository and worktree paths come from repos.yaml and from branch names, so
// they reach os.RemoveAll and git worktree add as configuration rather than as
// paths xr chose. The check lives here so there is one implementation of it
// rather than one per package.
//
// The check is lexical: it compares the two paths as text and never touches
// the filesystem. It therefore catches a path that spells its way out with
// "..", and does not catch one that leaves through a symlinked parent — if
// repos/link points outside the workspace, repos/link/x is still "inside"
// repos as far as Inside is concerned. Callers that must not follow a symlink
// out need to resolve the path themselves, or operate through an opened root
// (os.OpenRoot).
package pathsafe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Inside reports whether path is lexically contained within dir, as an error
// describing the escape when it is not. dir itself is not "inside" dir: a
// caller that is about to create or delete path would otherwise be handed the
// directory it is supposed to be confined to.
//
// It does not resolve symlinks — see the package comment for what that means
// for a caller relying on it.
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
	// Compare whole path elements: a plain "*.." prefix test would also reject a
	// sibling legitimately named "..foo".
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path %q escapes %q", path, dir)
	}
	return nil
}
