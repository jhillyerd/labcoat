package nix

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"text/template"

	"github.com/jhillyerd/labcoat/internal/config"
)

type NamesRequest struct {
	FlakePath string
}

func GetNames(data NamesRequest) ([]string, error) {
	flakeURL := fmt.Sprintf("git+file://%s#nixosConfigurations", data.FlakePath)
	output, err := nixEval(flakeURL, "builtins.attrNames")
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
	flakeURL := fmt.Sprintf("git+file://%s#nixosConfigurations.%s", data.FlakePath, data.HostName)

	// Render the --apply expression.
	var applyBuf bytes.Buffer
	if err := targetInfoApplyExpr.Execute(&applyBuf, struct{ AttrPath string }{
		AttrPath: data.Config.Hosts.DeployHostAttr,
	}); err != nil {
		return nil, fmt.Errorf("nix apply template render: %w", err)
	}
	applyExpr := applyBuf.String()

	output, err := nixEval(flakeURL, applyExpr)
	if err != nil {
		return nil, err
	}

	var targetInfo TargetInfo
	if err := json.Unmarshal(output, &targetInfo); err != nil {
		return nil, fmt.Errorf("nix decode failed: %w\n\nJSON output:\n%s", err, string(output))
	}

	return &targetInfo, nil
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
	cmd := exec.Command("nix", "flake", "metadata", "--json", "git+file://"+flakePath)
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
