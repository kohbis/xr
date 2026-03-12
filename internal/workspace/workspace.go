package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kohbis/xr/internal/config"
)

type Workspace struct {
	Config *config.Config
	Root   string
}

func New(root string, cfg *config.Config) *Workspace {
	return &Workspace{Root: root, Config: cfg}
}

func (w *Workspace) Init() error {
	if err := os.MkdirAll(w.Root, 0755); err != nil {
		return fmt.Errorf("creating workspace directory: %w", err)
	}

	if err := w.gitInit(); err != nil {
		return err
	}

	wsDir := filepath.Join(w.Root, w.Config.Workspace)
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		return fmt.Errorf("creating repos directory: %w", err)
	}

	if len(w.Config.Repositories) == 0 {
		if err := w.createReadme(); err != nil {
			return fmt.Errorf("creating README: %w", err)
		}
		return nil
	}

	for _, repo := range w.Config.Repositories {
		if err := w.addRepo(repo, wsDir); err != nil {
			return fmt.Errorf("adding repo %s: %w", repo.Name, err)
		}
	}

	return nil
}

func (w *Workspace) CreateGitignore(ignoreWorkspace bool) error {
	gitignorePath := filepath.Join(w.Root, ".gitignore")

	existing, _ := os.ReadFile(gitignorePath)
	entry := strings.TrimPrefix(w.Config.Workspace, "./") + "/"

	if ignoreWorkspace {
		if containsLine(string(existing), entry) {
			fmt.Printf("  %s is already in .gitignore\n", entry)
			return nil
		}
		content := string(existing)
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += entry + "\n"
		fmt.Printf("  adding %s to .gitignore\n", entry)
		return os.WriteFile(gitignorePath, []byte(content), 0644)
	}

	fmt.Println("  .gitignore unchanged")
	return nil
}

func containsLine(content, line string) bool {
	normalized := normalizeGitignoreLine(line)
	for _, l := range strings.Split(content, "\n") {
		if normalizeGitignoreLine(strings.TrimSpace(l)) == normalized {
			return true
		}
	}
	return false
}

func normalizeGitignoreLine(s string) string {
	s = strings.TrimPrefix(s, "./")
	s = strings.TrimPrefix(s, "/")
	return s
}

func (w *Workspace) createReadme() error {
	readmePath := filepath.Join(w.Root, "README.md")
	if _, err := os.Stat(readmePath); err == nil {
		return nil // already exists
	}
	content := "# Workspace\n\nInitialized by xr. Edit `repos.yaml` to add repositories, then run `xr init`.\n"
	fmt.Printf("  creating README.md\n")
	return os.WriteFile(readmePath, []byte(content), 0644)
}

func (w *Workspace) gitInit() error {
	gitDir := filepath.Join(w.Root, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return nil // already initialized
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = w.Root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	return nil
}

func (w *Workspace) addRepo(repo config.Repository, wsDir string) error {
	destPath := filepath.Join(wsDir, repo.Path)
	if repo.IsSymlink() {
		return w.addSymlink(repo, destPath)
	}
	if repo.IsClone() {
		return w.addClone(repo, destPath)
	}
	return w.addSubmodule(repo, destPath)
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func (w *Workspace) addSymlink(repo config.Repository, destPath string) error {
	if _, err := os.Lstat(destPath); err == nil {
		fmt.Printf("  symlink %s already exists, skipping\n", repo.Name)
		return nil
	}
	source := expandTilde(repo.Source)
	fmt.Printf("  creating symlink %s -> %s\n", repo.Name, source)
	if err := os.Symlink(source, destPath); err != nil {
		return fmt.Errorf("creating symlink: %w", err)
	}
	return nil
}

func (w *Workspace) addSubmodule(repo config.Repository, destPath string) error {
	if _, err := os.Stat(destPath); err == nil {
		fmt.Printf("  submodule %s already exists, skipping\n", repo.Name)
		return nil
	}

	relPath, err := filepath.Rel(w.Root, destPath)
	if err != nil {
		return fmt.Errorf("computing relative path: %w", err)
	}

	fmt.Printf("  adding submodule %s from %s\n", repo.Name, repo.Source)

	// -f is required because the workspace directory may be listed in .gitignore
	// (added by `xr gitignore`), which would otherwise prevent git submodule add.
	args := []string{"submodule", "add", "-f"}
	if repo.Branch != "" {
		args = append(args, "-b", repo.Branch)
	}
	args = append(args, repo.Source, relPath)

	cmd := exec.Command("git", args...)
	cmd.Dir = w.Root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git submodule add: %w", err)
	}
	return nil
}


func (w *Workspace) addClone(repo config.Repository, destPath string) error {
	if _, err := os.Stat(destPath); err == nil {
		fmt.Printf("  clone %s already exists, skipping\n", repo.Name)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}

	fmt.Printf("  cloning %s from %s\n", repo.Name, repo.Source)

	args := []string{"clone"}
	if repo.Branch != "" {
		args = append(args, "-b", repo.Branch)
	}
	args = append(args, repo.Source, destPath)

	cmd := exec.Command("git", args...)
	cmd.Dir = w.Root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	return nil
}

func (w *Workspace) Update(repoNames []string, pull bool) error {
	wsDir := filepath.Join(w.Root, w.Config.Workspace)

	for _, repo := range w.Config.Repositories {
		if len(repoNames) > 0 && !slices.Contains(repoNames, repo.Name) {
			continue
		}

		destPath := filepath.Join(wsDir, repo.Path)
		var err error
		switch {
		case repo.IsSymlink():
			err = w.updateSymlink(repo, destPath)
		case repo.IsClone():
			err = w.updateClone(repo, destPath, pull)
		default:
			err = w.updateSubmodule(repo, destPath, pull)
		}
		if err != nil {
			fmt.Printf("  warning: %s: %v\n", repo.Name, err)
		}
	}

	return nil
}

func (w *Workspace) updateSymlink(repo config.Repository, destPath string) error {
	info, err := os.Lstat(destPath)
	if err != nil {
		fmt.Printf("  symlink %s missing, recreating\n", repo.Name)
		return w.addSymlink(repo, destPath)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s exists but is not a symlink", destPath)
	}
	target, err := os.Readlink(destPath)
	if err != nil {
		return fmt.Errorf("reading symlink: %w", err)
	}
	source := expandTilde(repo.Source)
	if target == source {
		fmt.Printf("  symlink %s ok\n", repo.Name)
	} else {
		fmt.Printf("  symlink %s points to %s (expected %s)\n", repo.Name, target, source)
	}
	return nil
}

func (w *Workspace) updateSubmodule(repo config.Repository, destPath string, pull bool) error {
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return w.addSubmodule(repo, destPath)
	}

	fmt.Printf("  updating submodule %s\n", repo.Name)

	cmd := exec.Command("git", "submodule", "update", "--init", "--recursive")
	cmd.Dir = w.Root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git submodule update: %w", err)
	}

	if pull {
		fmt.Printf("  pulling %s\n", repo.Name)
		pullCmd := exec.Command("git", "pull", "origin", repo.Branch)
		pullCmd.Dir = destPath
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
		if err := pullCmd.Run(); err != nil {
			return fmt.Errorf("git pull: %w", err)
		}
	}

	return nil
}

func (w *Workspace) updateClone(repo config.Repository, destPath string, pull bool) error {
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return w.addClone(repo, destPath)
	}

	if pull {
		fmt.Printf("  pulling %s\n", repo.Name)
		args := []string{"pull", "origin"}
		if repo.Branch != "" {
			args = append(args, repo.Branch)
		}
		cmd := exec.Command("git", args...)
		cmd.Dir = destPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git pull: %w", err)
		}
	} else {
		fmt.Printf("  clone %s ok\n", repo.Name)
	}

	return nil
}

