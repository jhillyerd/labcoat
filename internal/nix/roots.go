package nix

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jhillyerd/labcoat/internal/config"
)

// RootsDir returns the directory where labcoat stores per-host GC root
// symlinks.  Config.Nix.RootsDir overrides the default, which lives under
// $XDG_STATE_HOME (falling back to ~/.local/state), next to the database
// file.  The directory itself is not created here.
func RootsDir(cfg config.Config) string {
	if cfg.Nix.RootsDir != "" {
		return cfg.Nix.RootsDir
	}

	stateRoot := os.Getenv("XDG_STATE_HOME")
	if stateRoot == "" {
		home := os.Getenv("HOME")
		if home == "" {
			// Nothing better available; likely will fail later with a clear
			// error from os.MkdirAll.
			stateRoot = ".local/state"
		} else {
			stateRoot = filepath.Join(home, ".local", "state")
		}
	}

	return filepath.Join(stateRoot, "labcoat", "roots")
}

// HostRootLink returns the path of the single GC root symlink for hostName
// inside rootsDir, named for the host itself, e.g. .../labcoat/roots/web01.
// The nix-store link name is arbitrary; GC tracking comes from the indirect
// registration, not the name.  hostName must pass validRootName.
func HostRootLink(rootsDir, hostName string) string {
	return filepath.Join(rootsDir, hostName)
}

// validRootName reports whether hostName is safe to embed as a single path
// element in a GC root link path.  Hostnames come from nixosConfigurations
// attrNames, which in practice are attrpath-safe (letters, digits, `-`,
// `_`), but a pathological name must never escape the roots directory.
func validRootName(hostName string) bool {
	return hostName != "" && hostName != "." && hostName != ".." &&
		!strings.ContainsAny(hostName, `\/`)
}

// nixStoreRunner registers storePath as a GC root at linkPath, atomically
// replacing any symlink already there.  Tests stub it to run without nix
// installed.
var nixStoreRunner = func(storePath, linkPath string) error {
	return exec.Command("nix-store", "--realise", storePath,
		"--add-root", linkPath, "--indirect").Run()
}

// RegisterRoot records storePath as hostName's only GC root by replacing the
// previous per-host root symlink.  nix-store itself atomically replaces an
// existing symlink at the link path (temp link plus rename), so there is no
// pre-delete here: the old closure stays rooted until the moment the new
// root is registered, and the old path becomes collectable only after nix
// garbage collection runs.  The indirect registration is keyed by the link
// path, so repeated deploys never accumulate extra roots.
//
// This prevents `nix-store --gc` on the control machine from deleting the
// closure of a deployed NixOS system (issue #17), because nixos-rebuild
// does not pass --add-root.
func RegisterRoot(rootsDir, hostName, storePath string) error {
	if !validRootName(hostName) {
		return fmt.Errorf("host name %q is not usable as a GC root link name", hostName)
	}

	if !filepath.IsAbs(storePath) {
		return fmt.Errorf("refusing to register non-store path as GC root: %q", storePath)
	}

	linkDir := filepath.Dir(HostRootLink(rootsDir, hostName))
	if err := os.MkdirAll(linkDir, 0700); err != nil {
		return fmt.Errorf("create roots dir: %w", err)
	}

	linkPath := HostRootLink(rootsDir, hostName)

	// Ensure the path exists in the local store, then create an indirect
	// root symlink pointing at it.  Realising an already-present path is a
	// no-op, so this costs nothing when the deploy built locally.
	slog.Debug("Registering GC root", "host", hostName,
		"path", storePath, "link", linkPath)
	if err := nixStoreRunner(storePath, linkPath); err != nil {
		return fmt.Errorf("nix-store --add-root failed: %w", err)
	}

	// Sanity check: --indirect must produce a symlink, not a real path.
	if fi, err := os.Lstat(linkPath); err != nil {
		return fmt.Errorf("GC root %s was not created: %w", linkPath, err)
	} else if fi.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("GC root %s is not a symlink", linkPath)
	}

	return nil
}
