// Package git provides shared helpers for running git commands from internal packages.
//
// Layering guideline:
//   - cmd/* should not execute git directly; call internal services instead.
//   - internal packages may use this package for git command execution/query helpers.
//   - prefer exported helpers (CurrentBranch, IsDirty, RemoteURL, RunQuiet, etc.)
//     over direct exec.Command("git", ...), so behavior stays consistent.
//
// File roles:
//   - execute.go: command runners (stdout/stderr/output/quiet variants)
//   - query.go: common repository queries (branch, remote, commit, ignore checks)
//   - status.go: __git_ps1-style status snapshot construction
//   - plumbing.go: lower-level internals used by query/status
package git
