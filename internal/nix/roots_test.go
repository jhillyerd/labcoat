package nix

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	want := filepath.Join("/roots", "web01")
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

func TestSystemOutPath(t *testing.T) {
	// nixRawEvalRunner stub overrides the `nix eval --raw` shell-out so the
	// test runs without nix.  Returns a cleanup func to restore.
	nixRawEvalStub := func(t *testing.T, fn func(url string) ([]byte, error)) {
		t.Helper()

		orig := nixRawEvalRunner
		nixRawEvalRunner = fn
		t.Cleanup(func() { nixRawEvalRunner = orig })
	}

	t.Run("evals toplevel outPath attrpath", func(t *testing.T) {
		// Regression: nixosConfigurations.<host> exposes NixOS options under
		// .config, so the derivation is config.system.build.toplevel.
		// E.g. nixosConfigurations.web01.system.outPath does not exist.
		nixRawEvalStub(t, func(url string) ([]byte, error) {
			want := "#nixosConfigurations.web01.config.system.build.toplevel.outPath"
			if !strings.HasSuffix(url, want) {
				t.Errorf("eval url = %q, want suffix %q", url, want)
			}
			return []byte("/nix/store/abc-nixos-system-web01-1.2.3\n"), nil
		})

		got, err := SystemOutPath(OutPathRequest{FlakePath: t.TempDir(), HostName: "web01"})
		require.NoError(t, err)
		assert.Equal(t, "/nix/store/abc-nixos-system-web01-1.2.3", got,
			"output must be trimmed")
	})

	t.Run("eval failure", func(t *testing.T) {
		nixRawEvalStub(t, func(url string) ([]byte, error) {
			return nil, &exec.ExitError{Stderr: []byte("error: attribute missing")}
		})

		_, err := SystemOutPath(OutPathRequest{FlakePath: t.TempDir(), HostName: "web01"})
		require.Error(t, err)
		assert.ErrorContains(t, err, "nix eval failed")
		assert.ErrorContains(t, err, "error: attribute missing")
	})
}

// emulateNixAddRoot mimics `nix-store --add-root ... --indirect` for tests:
// it replaces any existing symlink at linkPath with one pointing at
// storePath, the way nix does via a temp link and an atomic rename.
func emulateNixAddRoot(storePath, linkPath string) error {
	if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(storePath, linkPath)
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

	t.Run("rejects unusable host names", func(t *testing.T) {
		rootsDir := t.TempDir()
		nixStoreCommand(t, func(storePath, linkPath string) error {
			t.Fatal("nix-store should not run for invalid host name")
			return nil
		})

		for _, name := range []string{"", ".", "..", "a/b", "a\\b", "../evil"} {
			if err := RegisterRoot(rootsDir, name, "/nix/store/abc-system-1"); err == nil {
				t.Errorf("RegisterRoot(%q) = nil, want error", name)
			}
		}
	})

	t.Run("creates link dir and indirect root", func(t *testing.T) {
		rootsDir := t.TempDir()
		storePath := "/nix/store/abc123-nixos-system-host1-1.2.3"

		var gotStore, gotLink string
		nixStoreCommand(t, func(storePath, linkPath string) error {
			gotStore, gotLink = storePath, linkPath
			return emulateNixAddRoot(storePath, linkPath)
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
			return emulateNixAddRoot(storePath, linkPath)
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

		// Exactly one root per host: rootsDir holds only host1's link.
		entries, err := os.ReadDir(rootsDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name() != "host1" {
			t.Errorf("unexpected root dir entries: %v", entries)
		}
	})

	t.Run("failed registration keeps previous root", func(t *testing.T) {
		rootsDir := t.TempDir()
		oldPath := "/nix/store/aaa-old-system-1"

		link := HostRootLink(rootsDir, "host1")
		require.NoError(t, os.MkdirAll(filepath.Dir(link), 0700))
		require.NoError(t, os.Symlink(oldPath, link))

		nixStoreCommand(t, func(storePath, linkPath string) error {
			return os.ErrPermission
		})

		err := RegisterRoot(rootsDir, "host1", "/nix/store/bbb-new-system-2")
		require.Error(t, err)

		// The old root must survive a failed registration so the deployed
		// closure stays protected until the new root is in place.
		target, lerr := os.Readlink(link)
		require.NoError(t, lerr)
		assert.Equal(t, oldPath, target)
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
