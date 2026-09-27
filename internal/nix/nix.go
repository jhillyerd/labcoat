package nix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jhillyerd/labcoat/internal/config"
)

// IsGitRepo reports whether flakePath is the root of a git repository.
// Other packages use it to disable git-only features for non-git flake dirs.
// An empty or missing flakePath is reported as false without shelling out.
func IsGitRepo(flakePath string) bool {
	if flakePath == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(flakePath, ".git")); err != nil {
		return false
	}

	// A present .git entry is not sufficient: stale directories and broken
	// worktree/submodule pointers can leave one behind without a usable
	// repository.  Confirm git itself recognizes the tree.
	cmd := exec.Command("git", "-C", flakePath, "rev-parse", "--is-inside-work-tree")
	output, err := cmd.Output()
	if err != nil {
		slog.Debug("git rev-parse rejected flake dir", "path", flakePath, "err", err)
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

// flakeURL returns the nix flake URL to reference flakePath.  Git repos use
// git+file:// URLs so that gitignored directories are not traversed (see
// PR #43); paths that are not git repos fall back to a plain local path, so
// labcoat works with flake directories managed by other tools.  Git-only
// features (revision, dirty state) are unavailable in that case.
func flakeURL(flakePath, fragment string) string {
	if IsGitRepo(flakePath) {
		if fragment == "" {
			return fmt.Sprintf("git+file://%s", flakePath)
		}
		return fmt.Sprintf("git+file://%s#%s", flakePath, fragment)
	}
	if fragment == "" {
		return flakePath
	}
	return fmt.Sprintf("%s#%s", flakePath, fragment)
}

type NamesRequest struct {
	FlakePath string
}

func GetNames(data NamesRequest) ([]string, error) {
	url := flakeURL(data.FlakePath, "nixosConfigurations")
	output, err := nixEval(url, "builtins.attrNames")
	if err != nil {
		return nil, err
	}

	var names []string
	if err := json.Unmarshal(output, &names); err != nil {
		return nil, fmt.Errorf("nix decode failed: %w\n\nJSON output:\n%s", err, string(output))
	}

	return names, nil
}

var targetInfoApplyExpr = template.Must(
	template.New("targetInfoApply").Parse("target: { deployHost = {{ .AttrPath }}; }"))

type TargetInfoRequest struct {
	FlakePath string
	HostName  string
	Config    config.Config
}

// TargetInfo contains host information queried from nix.  It is cached.
type TargetInfo struct {
	DeployHost string `json:"deployHost"`
	DeployUser string `json:"deployUser"`
}

func (ti *TargetInfo) SSHDestination() string {
	user := ti.DeployUser

	dest := "ssh://"
	if user != "" {
		dest += user + "@"
	}
	dest += ti.DeployHost

	return dest
}

func GetTargetInfo(data TargetInfoRequest) (*TargetInfo, error) {
	url := flakeURL(data.FlakePath, fmt.Sprintf("nixosConfigurations.%s", data.HostName))

	// Render the --apply expression.
	var applyBuf bytes.Buffer
	if err := targetInfoApplyExpr.Execute(&applyBuf, struct{ AttrPath string }{
		AttrPath: data.Config.Hosts.DeployHostAttr,
	}); err != nil {
		return nil, fmt.Errorf("nix apply template render: %w", err)
	}
	applyExpr := applyBuf.String()

	output, err := nixEval(url, applyExpr)
	if err != nil {
		return nil, err
	}

	var targetInfo TargetInfo
	if err := json.Unmarshal(output, &targetInfo); err != nil {
		return nil, fmt.Errorf("nix decode failed: %w\n\nJSON output:\n%s", err, string(output))
	}

	return &targetInfo, nil
}

type OutPathRequest struct {
	FlakePath string
	HostName  string
}

// SystemOutPath returns the Nix store path of the system derivation for
// hostName in flakePath, e.g. /nix/store/abc-nixos-system-host-1.2.3. This is
// the path that must be registered as a GC root after deployment so that
// garbage collection cannot delete the running closure.  ctx bounds the
// evaluation subprocess, canceling it if it stalls.
//
// Note the eval result of lib.nixosSystem exposes NixOS options under
// .config, so the derivation nixos-rebuild activates is
// nixosConfigurations.<host>.config.system.build.toplevel; the top-level
// result has no .system attribute.
func SystemOutPath(ctx context.Context, data OutPathRequest) (string, error) {
	url := flakeURL(data.FlakePath,
		fmt.Sprintf("nixosConfigurations.%s.config.system.build.toplevel.outPath", data.HostName))

	// Not nixEval: --raw returns the string itself, not JSON-quoted.
	slog.Debug("Running nix eval (raw)", "url", url)
	output, err := nixRawEvalRunner(ctx, url)
	if err != nil {
		stderr := ""
		if exit, ok := err.(*exec.ExitError); ok {
			stderr = "\n\nOutput:\n" + string(exit.Stderr)
		}

		return "", fmt.Errorf("nix eval failed: %w\n\nURL: %s%s", err, url, stderr)
	}

	return strings.TrimSpace(string(output)), nil
}

type FlakeMetadata struct {
	ResolvedUrl  string
	Revision     string
	Dirty        bool
	LastModified int64
	Fingerprint  string
}

type flakeMetadataJSON struct {
	ResolvedUrl   string `json:"resolvedUrl"`
	Revision      string `json:"revision"`
	DirtyRevision string `json:"dirtyRevision"`
	LastModified  int64  `json:"lastModified"`
	Fingerprint   string `json:"fingerprint"`
}

func GetFlakeMetadata(flakePath string) (*FlakeMetadata, error) {
	cmd := exec.Command("nix", "flake", "metadata", "--json", flakeURL(flakePath, ""))
	output, err := cmd.Output()
	if err != nil {
		out := ""
		if exit, ok := err.(*exec.ExitError); ok {
			out = "\n\nOutput:\n" + string(exit.Stderr)
		}
		return nil, fmt.Errorf("nix flake metadata failed: %w%s", err, out)
	}

	var raw flakeMetadataJSON
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("nix flake metadata decode failed: %w", err)
	}

	meta := &FlakeMetadata{
		ResolvedUrl:  raw.ResolvedUrl,
		LastModified: raw.LastModified,
		Fingerprint:  raw.Fingerprint,
	}

	if raw.DirtyRevision != "" {
		meta.Revision = raw.DirtyRevision
		meta.Dirty = true
	} else {
		meta.Revision = raw.Revision
	}

	return meta, nil
}

// nixRawEvalRunner shells out to `nix eval --raw` for url, killing the
// subprocess when ctx is done.  Tests stub it to run without nix installed.
// --no-eval-cache keeps this eval from contending on the per-flake eval-cache
// database with concurrently running nix processes (e.g. nixos-rebuild at
// deploy start), which otherwise emits "error (ignored): SQLite database ...
// is busy" noise into their output.
var nixRawEvalRunner = func(ctx context.Context, url string) ([]byte, error) {
	return exec.CommandContext(ctx, "nix", "eval", "--raw", "--no-eval-cache", url).Output()
}

func nixEval(flakeURL string, applyExpr string) ([]byte, error) {
	slog.Debug("Running nix eval", "url", flakeURL, "apply", applyExpr)

	args := []string{"eval", "--json", flakeURL}
	if applyExpr != "" {
		args = append(args, "--apply", applyExpr)
	}
	cmd := exec.Command("nix", args...)

	output, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exit, ok := err.(*exec.ExitError); ok {
			stderr = "\n\nOutput:\n" + string(exit.Stderr)
		}

		return nil, fmt.Errorf("nix eval failed: %w\n\nURL: %s\nApply: %s%s", err, flakeURL, applyExpr, stderr)
	}

	return output, nil
}
