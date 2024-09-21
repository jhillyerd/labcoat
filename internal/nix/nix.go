package nix

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"text/template"

	"github.com/jhillyerd/labcoat/internal/config"
)

const namesScript = `
	let
		flake = builtins.getFlake "path:{{ .FlakePath }}";
	in
	builtins.attrNames flake.nixosConfigurations
`

var namesTmpl = template.Must(template.New("names").Parse(namesScript))

type NamesRequest struct {
	FlakePath string
}

func GetNames(data NamesRequest) ([]string, error) {
	output, err := runScript(namesTmpl, data)
	if err != nil {
		return nil, err
	}

	var names []string
	if err := json.Unmarshal(output, &names); err != nil {
		return nil, fmt.Errorf("nix decode failed: %w\n\nJSON output:\n%s", err, string(output))
	}

	return names, nil
}

const targetInfoScript = `
	let
		flake = builtins.getFlake "path:{{ .FlakePath }}";
		key = "{{ .HostName }}";
		target = flake.nixosConfigurations.${key};
	in
	{
		deployHost = {{ .Config.Hosts.DeployHostAttr }};
	}
`

var targetInfoTmpl = template.Must(template.New("targetInfo").Parse(targetInfoScript))

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
	output, err := runScript(targetInfoTmpl, data)
	if err != nil {
		return nil, err
	}

	var targetInfo TargetInfo
	if err := json.Unmarshal(output, &targetInfo); err != nil {
		return nil, fmt.Errorf("nix decode failed: %w\n\nJSON output:\n%s", err, string(output))
	}

	return &targetInfo, nil
}

func runScript(tmpl *template.Template, data any) ([]byte, error) {
	// Render script.
	var scriptBuf bytes.Buffer
	if err := tmpl.Execute(&scriptBuf, data); err != nil {
		return nil, fmt.Errorf("nix template render: %w", err)
	}
	script, err := io.ReadAll(&scriptBuf)
	if err != nil {
		return nil, fmt.Errorf("nix template read: %w", err)
	}
	slog.Debug("Running nix script", "script", script)

	// Pass script to nix cmd.
	cmd := exec.Command("nix", "eval", "--file", "-", "--json")
	cmd.Stdin = bytes.NewReader(script)

	output, err := cmd.Output()
	if err != nil {
		output := ""
		if exit, ok := err.(*exec.ExitError); ok {
			output = "\n\nOutput:\n"
			output += string(exit.Stderr)
		}

		return nil, fmt.Errorf("nix run failed: %w\n\nScript:\n%s%s", err, string(script), output)
	}

	return output, nil
}
