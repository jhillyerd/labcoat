package nix

import (
	"bytes"
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
	// Render script.
	var scriptBuf bytes.Buffer
	if err := namesTmpl.Execute(&scriptBuf, data); err != nil {
		return nil, newError(err, []byte(namesScript))
	}
	script, err := io.ReadAll(&scriptBuf)
	if err != nil {
		return nil, newError(err, []byte(namesScript))
	}

	// Pass script to nix cmd.
	cmd := exec.Command("nix", "eval", "--file", "-", "--json")
	cmd.Stdin = bytes.NewReader(script)

	output, err := cmd.Output()
	if err != nil {
		return nil, newError(err, script)
	}

	return output, nil
}

type Error struct {
	Cause  error
	Script []byte
}

func newError(cause error, script []byte) *Error {
	return &Error{
		Cause:  cause,
		Script: script,
	}
}

func (e Error) Error() string {
	return e.Cause.Error()
}

func (e Error) Detail() string {
	var out string

	switch err := e.Cause.(type) {
	case *exec.ExitError:
		out += fmt.Sprintf("nix failed, exit code %v\n", err.ExitCode())
		out += string(err.Stderr)
	default:
		out += fmt.Sprintf("nix failed, error: %v\n", err)
	}

	if e.Script != nil {
		out += "\n\nscript source:\n"
		out += string(e.Script)
	}

	return out
}
