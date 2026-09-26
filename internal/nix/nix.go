package nix

import (
	"bytes"
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
func IsGitRepo(flakePath string) bool {
	if _, err := os.Stat(filepath.Join(flakePath, ".git")); err == nil {
		return true
	}
	return false
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
// garbage collection cannot delete the running closure.
func SystemOutPath(data OutPathRequest) (string, error) {
	url := flakeURL(data.FlakePath,
		fmt.Sprintf("nixosConfigurations.%s.system.outPath", data.HostName))

	// Not nixEval: --raw returns the string itself, not JSON-quoted.
	slog.Debug("Running nix eval (raw)", "url", url)
	cmd := exec.Command("nix", "eval", "--raw", url)

	output, err := cmd.Output()
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
