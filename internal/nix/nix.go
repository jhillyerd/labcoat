package nix

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"text/template"
)

const namesScript = `
	let
		flake = builtins.getFlake "path:{{ .FlakePath }}";
	in
	builtins.attrNames flake.nixosConfigurations
`

var namesTmpl = template.Must(template.New("names").Parse(namesScript))

type NamesData struct {
	FlakePath string
}

func RunNames(data NamesData) ([]byte, *Error) {
	return runScript(namesTmpl, data)
}

const targetInfoScript = `
	let
		flake = builtins.getFlake "path:{{ .FlakePath }}";
		key = "{{ .HostName }}";
		target = flake.nixosConfigurations.${key};
	in
	{
		deployHost = target.config.networking.fqdnOrHostName;
	}
`

var targetInfoTmpl = template.Must(template.New("targetInfo").Parse(targetInfoScript))

type TargetInfoRequest struct {
	FlakePath string
	HostName  string
}

// TargetInfo contains host information queried from nix.  It is cached.
type TargetInfo struct {
	DeployHost string `json:"deployHost"`
}

func GetTargetInfo(data TargetInfoRequest) (*TargetInfo, *Error) {
	output, err := runScript(targetInfoTmpl, data)
	if err != nil {
		return nil, err
	}

	var targetInfo TargetInfo
	if err := json.Unmarshal(output, &targetInfo); err != nil {
		return nil, newJSONError(err, output)
	}

	return &targetInfo, nil
}

func runScript(tmpl *template.Template, data any) ([]byte, *Error) {
	// Render script.
	var scriptBuf bytes.Buffer
	if err := tmpl.Execute(&scriptBuf, data); err != nil {
		return nil, newNixError(err, []byte(namesScript))
	}
	script, err := io.ReadAll(&scriptBuf)
	if err != nil {
		return nil, newNixError(err, []byte(namesScript))
	}

	// Pass script to nix cmd.
	cmd := exec.Command("nix", "eval", "--file", "-", "--json")
	cmd.Stdin = bytes.NewReader(script)

	output, err := cmd.Output()
	if err != nil {
		return nil, newNixError(err, script)
	}

	return output, nil
}

type Error struct {
	Cause  error
	Script []byte
	JSON   []byte
}

func newNixError(cause error, script []byte) *Error {
	return &Error{
		Cause:  cause,
		Script: script,
	}
}

func newJSONError(cause error, json []byte) *Error {
	return &Error{
		Cause: cause,
		JSON:  json,
	}
}

func (e Error) Error() string {
	return e.Cause.Error()
}

func (e Error) Detail() string {
	var out string

	switch err := e.Cause.(type) {
	case *exec.ExitError:
		out = fmt.Sprintf("nix failed, exit code %v\n", err.ExitCode())
		out += string(err.Stderr)

	case *json.UnmarshalTypeError, *json.SyntaxError:
		out = fmt.Sprintf("JSON decode failed: %s", err.Error())

	default:
		out += fmt.Sprintf("nix run failed, %T: %v\n", err, err)
	}

	if e.Script != nil {
		out += "\n\nscript source:\n"
		out += string(e.Script)
	}

	if e.JSON != nil {
		out += "\n\nJSON source:\n"
		out += string(e.JSON)
	}

	return out
}
