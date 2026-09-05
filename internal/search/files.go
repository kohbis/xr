package search

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kohbis/xr/internal/git"
)

// binarySniffLen is how much of a file is inspected for a NUL byte before it is
// treated as binary. ripgrep uses a comparable prefix scan.
const binarySniffLen = 8000

// listFiles returns the paths, relative to repoPath and slash-separated, that a
// search should cover, filtered by glob and sorted.
//
// The file set comes from git (tracked files plus untracked files that are not
// ignored), so ignored build output and dependency trees never take part in a
// search. Outside a git repository it falls back to walking the directory,
// skipping .git. Both search engines are handed this same list, which is what
// keeps results identical whether or not ripgrep is installed.
func listFiles(repoPath, glob string) ([]string, error) {
	files, err := git.ListFiles(repoPath)
	if err != nil {
		files, err = walkFiles(repoPath)
		if err != nil {
			return nil, err
		}
	}

	kept := files[:0]
	for _, rel := range files {
		ok, err := matchGlob(glob, rel)
		if err != nil {
			return nil, err
		}
		if ok {
			kept = append(kept, rel)
		}
	}
	slices.Sort(kept)
	return kept, nil
}

// walkFiles lists the regular files under root, relative and slash-separated,
// for directories that git does not manage.
func walkFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files, err
}

// matchGlob reports whether the slash-separated path rel satisfies glob, using
// the rule ripgrep and .gitignore share: a pattern without a separator matches
// the file name at any depth ("*.go"), while a pattern containing one matches
// the whole relative path ("cmd/*.go"). A leading "**/" is redundant under that
// rule and is dropped. An empty glob matches everything.
func matchGlob(glob, rel string) (bool, error) {
	if glob == "" {
		return true, nil
	}
	glob = strings.TrimPrefix(glob, "**/")
	if !strings.Contains(glob, "/") {
		return path.Match(glob, path.Base(rel))
	}
	return path.Match(glob, rel)
}

// isBinary reports whether data looks like binary content, i.e. it contains a
// NUL byte in the inspected prefix. Binary files are left out of results by
// both engines: ripgrep reports them as a note rather than as matching lines,
// so skipping them keeps the two in step.
func isBinary(data []byte) bool {
	if len(data) > binarySniffLen {
		data = data[:binarySniffLen]
	}
	return bytes.IndexByte(data, 0) >= 0
}
