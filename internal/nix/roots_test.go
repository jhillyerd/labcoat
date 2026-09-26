package nix

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jhillyerd/labcoat/internal/config"
)

func TestRootsDir(t *testing.T) {
	t.Setenv("HOME", "/home/tester")

	t.Run("config override wins", func(t *testing.T) {
		cfg := config.Default()
		cfg.Nix.RootsDir = "/srv/labcoat-roots"
		if got := RootsDir(cfg); got != "/srv/labcoat-roots" {
			t.Errorf("RootsDir = %q, want config override", got)
		}
	})

	t.Run("XDG_STATE_HOME default", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "/state")
		cfg := config.Default()
		want := filepath.Join("/state", "labcoat", "roots")
		if got := RootsDir(cfg); got != want {
			t.Errorf("RootsDir = %q, want %q", got, want)
		}
	})

	t.Run("HOME fallback default", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "")
		cfg := config.Default()
		want := filepath.Join("/home/tester", ".local", "state", "labcoat", "roots")
		if got := RootsDir(cfg); got != want {
			t.Errorf("RootsDir = %q, want %q", got, want)
		}
	})
}

func TestHostRootLink(t *testing.T) {
	want := filepath.Join("/roots", "hosts", "web01", "result")
	if got := HostRootLink("/roots", "web01"); got != want {
		t.Errorf("HostRootLink = %q, want %q", got, want)
	}
}

// nixStoreCommand overrides how RegisterRoot invokes nix-store; tests set it
// to a stub.  Returns a cleanup func to restore.
func nixStoreCommand(t *testing.T, fn func(storePath, linkPath string) error) {
	t.Helper()

	orig := nixStoreRunner
	nixStoreRunner = fn
	t.Cleanup(func() { nixStoreRunner = orig })
}

func TestRegisterRoot(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("no /usr/bin/true available")
	}

	t.Run("rejects non-store path", func(t *testing.T) {
		rootsDir := t.TempDir()
		nixStoreCommand(t, func(storePath, linkPath string) error {
			t.Fatal("nix-store should not run for invalid path")
			return nil
		})

		if err := RegisterRoot(rootsDir, "host1", "relative/path"); err == nil {
			t.Error("RegisterRoot(relative) = nil, want error")
		}
	})

	t.Run("creates link dir and indirect root", func(t *testing.T) {
		rootsDir := t.TempDir()
		storePath := "/nix/store/abc123-nixos-system-host1-1.2.3"

		var gotStore, gotLink string
		nixStoreCommand(t, func(storePath, linkPath string) error {
			gotStore, gotLink = storePath, linkPath
			// Emulate `nix-store --add-root ... --indirect`: symlink
			// pointing at the store path.
			return os.Symlink(storePath, linkPath)
		})

		if err := RegisterRoot(rootsDir, "host1", storePath); err != nil {
			t.Fatalf("RegisterRoot: %v", err)
		}

		link := HostRootLink(rootsDir, "host1")
		target, err := os.Readlink(link)
		if err != nil {
			t.Fatalf("root symlink missing: %v", err)
		}
		if target != storePath {
			t.Errorf("root symlink -> %q, want %q", target, storePath)
		}
		if gotLink != link {
			t.Errorf("register link = %q, want %q", gotLink, link)
		}
		if gotStore != storePath {
			t.Errorf("register store path = %q, want %q", gotStore, storePath)
		}
	})

	t.Run("replaces previous root, one per host", func(t *testing.T) {
		rootsDir := t.TempDir()
		oldPath := "/nix/store/aaa-old-system-1"
		newPath := "/nix/store/bbb-new-system-2"

		link := HostRootLink(rootsDir, "host1")
		if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(oldPath, link); err != nil {
			t.Fatal(err)
		}

		nixStoreCommand(t, func(storePath, linkPath string) error {
			return os.Symlink(storePath, linkPath)
		})

		if err := RegisterRoot(rootsDir, "host1", newPath); err != nil {
			t.Fatalf("RegisterRoot: %v", err)
		}

		target, err := os.Readlink(link)
		if err != nil {
			t.Fatalf("root symlink missing after replacement: %v", err)
		}
		if target != newPath {
			t.Errorf("root symlink -> %q, want new path %q", target, newPath)
		}

		// Exactly one host dir must exist with exactly one root.
		hostDirs, err := os.ReadDir(filepath.Join(rootsDir, "hosts"))
		if err != nil {
			t.Fatal(err)
		}
		if len(hostDirs) != 1 || hostDirs[0].Name() != "host1" {
			t.Errorf("unexpected host root dirs: %v", hostDirs)
		}
		roots, err := os.ReadDir(filepath.Join(rootsDir, "hosts", "host1"))
		if err != nil {
			t.Fatal(err)
		}
		if len(roots) != 1 {
			t.Errorf("host has %d roots, want exactly 1", len(roots))
		}
	})

	t.Run("nix-store failure leaves no false success", func(t *testing.T) {
		rootsDir := t.TempDir()
		nixStoreCommand(t, func(storePath, linkPath string) error {
			return os.ErrPermission
		})

		err := RegisterRoot(rootsDir, "host1", "/nix/store/abc-system-1")
		if err == nil {
			t.Fatal("RegisterRoot = nil, want error when nix-store fails")
		}

		if _, statErr := os.Lstat(HostRootLink(rootsDir, "host1")); statErr == nil {
			t.Error("root symlink exists despite nix-store failure")
		}
	})

	t.Run("rejects real-file root (non-symlink)", func(t *testing.T) {
		rootsDir := t.TempDir()
		nixStoreCommand(t, func(storePath, linkPath string) error {
			// Emulate --indirect omitted: a real file instead of a link.
			return os.WriteFile(linkPath, []byte("not a link"), 0600)
		})

		err := RegisterRoot(rootsDir, "host1", "/nix/store/abc-system-1")
		if err == nil {
			t.Fatal("RegisterRoot = nil, want error for non-symlink root")
		}
	})
}
