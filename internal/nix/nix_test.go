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

	// A linked worktree has a .git pointer file and must count as a repo.
	repoDir := t.TempDir()
	gitInit(t, repoDir)
	if err := os.WriteFile(filepath.Join(repoDir, "flake.nix"), []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repoDir, "add", "flake.nix").Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := exec.Command("git", "-C", repoDir,
		"-c", "user.email=labcoat@example.com", "-c", "user.name=labcoat",
		"commit", "-q", "-m", "init").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	worktreeDir := filepath.Join(t.TempDir(), "wt")
	if err := exec.Command("git", "-C", repoDir, "worktree", "add", "-q", worktreeDir).Run(); err != nil {
		t.Fatalf("git worktree add: %v", err)
	}
	if !IsGitRepo(worktreeDir) {
		t.Errorf("IsGitRepo(%q) = false, want true for linked worktree", worktreeDir)
	}

	// A .git file with a broken pointer is NOT a usable repo: the false
	// positive here would route the dir back to git+file:// URLs.
	staleDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staleDir, ".git"), []byte("gitdir: /nonexistent-gitdir\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if IsGitRepo(staleDir) {
		t.Errorf("IsGitRepo(%q) = true, want false for broken .git pointer", staleDir)
	}

	// A .git directory that is not a real repository (stale dir) is not a repo.
	fakeDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(fakeDir, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if IsGitRepo(fakeDir) {
		t.Errorf("IsGitRepo(%q) = true, want false for fake .git dir", fakeDir)
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

	// A stale .git entry (broken pointer) must fall back to the plain path,
	// not git+file://, otherwise nix fails with "not a git repository".
	staleDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staleDir, ".git"), []byte("gitdir: /nonexistent-gitdir\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := flakeURL(staleDir, "nixosConfigurations"); got != staleDir+"#nixosConfigurations" {
		t.Errorf("stale-.git flake dir produced URL %q, want plain path (issue #49)", got)
	}
}
