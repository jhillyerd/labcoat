package nix

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitInit initializes a repo in dir, skipping when git is unavailable.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}

func TestIsGitRepo(t *testing.T) {
	// A plain directory is not a git repo.
	dir := t.TempDir()
	if IsGitRepo(dir) {
		t.Errorf("IsGitRepo(%q) = true, want false for plain dir", dir)
	}

	// A git repository is a git repo.
	gitInit(t, dir)
	if !IsGitRepo(dir) {
		t.Errorf("IsGitRepo(%q) = false, want true after git init", dir)
	}

	// A .git file (worktree/submodule pointer) still counts.
	fileDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(fileDir, ".git"), []byte("gitdir: /elsewhere\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if !IsGitRepo(fileDir) {
		t.Errorf("IsGitRepo(%q) = false, want true for .git file", fileDir)
	}

	// A nonexistent path is not a git repo.
	if IsGitRepo(filepath.Join(t.TempDir(), "missing")) {
		t.Error("IsGitRepo(nonexistent) = true, want false")
	}
}

func TestFlakeURL(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) string // flakePath under test
		fragment string
		want     func(dir string) string
	}{
		{
			name: "git repo with fragment",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				gitInit(t, dir)
				return dir
			},
			fragment: "nixosConfigurations",
			want:     func(dir string) string { return "git+file://" + dir + "#nixosConfigurations" },
		},
		{
			name: "git repo without fragment",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				gitInit(t, dir)
				return dir
			},
			want: func(dir string) string { return "git+file://" + dir },
		},
		{
			name: "non-git dir with fragment",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			fragment: "nixosConfigurations",
			want:     func(dir string) string { return dir + "#nixosConfigurations" },
		},
		{
			name: "non-git dir without fragment",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			want: func(dir string) string { return dir },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)
			got := flakeURL(dir, tt.fragment)
			if want := tt.want(dir); got != want {
				t.Errorf("flakeURL(%q, %q) = %q, want %q", dir, tt.fragment, got, want)
			}
		})
	}
}

// TestFlakeURLNoGitRegression is the regression test for issue #49: a
// flake-dir that is not a git repository (e.g. managed by jujutsu) must get
// a plain local flake reference instead of a git+file:// URL, otherwise nix
// fails with "not a git repository" and labcoat exits at startup.
func TestFlakeURLNoGitRegression(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "flake.nix"), []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if got := flakeURL(dir, "nixosConfigurations"); got != dir+"#nixosConfigurations" {
		t.Errorf("non-git flake dir produced URL %q, want plain path %q (issue #49)",
			got, dir+"#nixosConfigurations")
	}
	if got := flakeURL(dir, ""); got != dir {
		t.Errorf("non-git flake dir metadata URL = %q, want plain path %q (issue #49)", got, dir)
	}
}
