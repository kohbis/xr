package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "repos.yaml")
	content := `workspace: ./my-repos
repositories:
  - name: proj-a
    source: git@github.com:user/proj-a.git
    branch: main
    path: proj-a
  - name: local-lib
    source: /path/to/local-lib
    path: local-lib
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Workspace != "./my-repos" {
		t.Errorf("Workspace = %q, want %q", cfg.Workspace, "./my-repos")
	}
	if len(cfg.Repositories) != 2 {
		t.Fatalf("len(Repositories) = %d, want 2", len(cfg.Repositories))
	}

	if cfg.Repositories[0].Type != RepoTypeGit {
		t.Errorf("repo[0].Type = %q, want %q", cfg.Repositories[0].Type, RepoTypeGit)
	}
	if cfg.Repositories[1].Type != RepoTypeSymlink {
		t.Errorf("repo[1].Type = %q, want %q", cfg.Repositories[1].Type, RepoTypeSymlink)
	}
}

func TestLoad_DefaultWorkspace(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "repos.yaml")
	content := `repositories: []
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Workspace != "./repos" {
		t.Errorf("Workspace = %q, want %q", cfg.Workspace, "./repos")
	}
}

func TestLoad_TypeInference(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantType RepoType
	}{
		{"git SSH URL", "git@github.com:user/repo.git", RepoTypeGit},
		{"HTTPS URL", "https://github.com/user/repo.git", RepoTypeGit},
		{"absolute path", "/home/user/local-repo", RepoTypeSymlink},
		{"tilde path", "~/projects/repo", RepoTypeSymlink},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "repos.yaml")
			content := "repositories:\n  - name: test\n    source: " + tt.source + "\n"
			if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, err := Load(cfgPath)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.Repositories[0].Type != tt.wantType {
				t.Errorf("Type = %q, want %q", cfg.Repositories[0].Type, tt.wantType)
			}
		})
	}
}

func TestLoad_ExplicitTypePreserved(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "repos.yaml")
	content := `repositories:
  - name: test
    source: git@github.com:user/repo.git
    type: clone
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Repositories[0].Type != RepoTypeClone {
		t.Errorf("Type = %q, want %q", cfg.Repositories[0].Type, RepoTypeClone)
	}
}

func TestLoad_UnknownTypeReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "repos.yaml")
	content := `repositories:
  - name: test
    source: somewhere
    type: invalid
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() expected error for unknown type, got nil")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/repos.yaml")
	if err == nil {
		t.Fatal("Load() expected error for missing file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "repos.yaml")
	if err := os.WriteFile(cfgPath, []byte("{{not yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() expected error for invalid YAML, got nil")
	}
}

func TestLoad_EmptyPathDefaultsToName(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "repos.yaml")
	content := `repositories:
  - name: my-repo
    source: git@github.com:user/repo.git
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Repositories[0].Path != "my-repo" {
		t.Errorf("Path = %q, want %q", cfg.Repositories[0].Path, "my-repo")
	}
}

func TestRepository_IsSymlink(t *testing.T) {
	r := &Repository{Type: RepoTypeSymlink}
	if !r.IsSymlink() {
		t.Error("IsSymlink() = false, want true")
	}

	r.Type = RepoTypeGit
	if r.IsSymlink() {
		t.Error("IsSymlink() = true for git type, want false")
	}
}

func TestRepository_IsClone(t *testing.T) {
	r := &Repository{Type: RepoTypeClone}
	if !r.IsClone() {
		t.Error("IsClone() = false, want true")
	}

	r.Type = RepoTypeGit
	if r.IsClone() {
		t.Error("IsClone() = true for git type, want false")
	}
}

func TestSaveAndLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "repos.yaml")

	original := &Config{
		Workspace: "./ws",
		Repositories: []Repository{
			{Name: "a", Source: "git@github.com:u/a.git", Branch: "main", Path: "a", Type: RepoTypeGit},
			{Name: "b", Source: "/local/b", Path: "b", Type: RepoTypeSymlink},
		},
	}

	if err := Save(cfgPath, original); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Workspace != original.Workspace {
		t.Errorf("Workspace = %q, want %q", loaded.Workspace, original.Workspace)
	}
	if len(loaded.Repositories) != len(original.Repositories) {
		t.Fatalf("len(Repositories) = %d, want %d", len(loaded.Repositories), len(original.Repositories))
	}
	for i, repo := range loaded.Repositories {
		if repo.Name != original.Repositories[i].Name {
			t.Errorf("repo[%d].Name = %q, want %q", i, repo.Name, original.Repositories[i].Name)
		}
		if repo.Type != original.Repositories[i].Type {
			t.Errorf("repo[%d].Type = %q, want %q", i, repo.Type, original.Repositories[i].Type)
		}
	}
}
