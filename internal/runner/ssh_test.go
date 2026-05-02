package runner

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifySSHError(t *testing.T) {
	tcs := map[string]struct {
		output      string
		execErr     error
		wantMessage string
		wantHasDest bool
	}{
		"host key": {
			output:      "Host key verification failed.",
			wantMessage: "Host key not trusted",
			wantHasDest: true,
		},
		"permission denied": {
			output:      "Permission denied (publickey).",
			wantMessage: "SSH authentication failed",
		},
		"connection refused": {
			output:      "ssh: connect to host example.com port 22: Connection refused",
			wantMessage: "Connection refused",
		},
		"connection timed out": {
			output:      "ssh: connect to host example.com port 22: Connection timed out",
			wantMessage: "Connection timed out",
		},
		"no route": {
			output:      "ssh: connect to host example.com port 22: No route to host",
			wantMessage: "No route to host",
		},
		"resolve hostname": {
			output:      "ssh: Could not resolve hostname badhost: Name or service not known",
			wantMessage: "Could not resolve hostname",
		},
		"unknown error with output": {
			output:      "something unexpected happened",
			wantMessage: "SSH connection failed",
		},
		"unknown error empty": {
			output:      "",
			execErr:     fmt.Errorf("some exec error"),
			wantMessage: "SSH connection failed",
		},
		"ssh not found": {
			output:      "",
			execErr:     fmt.Errorf("exec: \"ssh\": executable file not found in $PATH"),
			wantMessage: "SSH command not found",
		},
		"context deadline": {
			output:      "",
			execErr:     fmt.Errorf("context deadline exceeded"),
			wantMessage: "SSH check timed out",
		},
		"signal killed": {
			output:      "",
			execErr:     fmt.Errorf("signal: killed"),
			wantMessage: "SSH check timed out",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			err := classifySSHError("user@host", tc.output, tc.execErr)
			assert.Equal(t, tc.wantMessage, err.Message)
			assert.NotEmpty(t, err.Suggestion)
			if tc.wantHasDest {
				assert.Contains(t, err.Suggestion, "user@host")
			}
		})
	}
}

func TestClassifySSHErrorHostKeyContainsDest(t *testing.T) {
	err := classifySSHError("root@myserver.example.com", "Host key verification failed.", nil)
	assert.Contains(t, err.Suggestion, "root@myserver.example.com")
}

func TestClassifySSHErrorSSHNotFoundSuggestion(t *testing.T) {
	err := classifySSHError("user@host", "",
		fmt.Errorf("exec: \"ssh\": executable file not found in $PATH"))
	assert.Equal(t, "SSH command not found", err.Message)
	assert.Contains(t, err.Suggestion, "ssh")
}

