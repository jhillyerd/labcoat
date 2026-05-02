package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifySSHError(t *testing.T) {
	tcs := map[string]struct {
		output      string
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
			wantHasDest: false,
		},
		"connection refused": {
			output:      "ssh: connect to host example.com port 22: Connection refused",
			wantMessage: "Connection refused",
			wantHasDest: false,
		},
		"connection timed out": {
			output:      "ssh: connect to host example.com port 22: Connection timed out",
			wantMessage: "Connection timed out",
			wantHasDest: false,
		},
		"no route": {
			output:      "ssh: connect to host example.com port 22: No route to host",
			wantMessage: "No route to host",
			wantHasDest: false,
		},
		"resolve hostname": {
			output:      "ssh: Could not resolve hostname badhost: Name or service not known",
			wantMessage: "Could not resolve hostname",
			wantHasDest: false,
		},
		"unknown error": {
			output:      "something unexpected happened",
			wantMessage: "SSH connection failed",
			wantHasDest: false,
		},
		"empty error": {
			output:      "",
			wantMessage: "SSH connection failed",
			wantHasDest: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			err := classifySSHError("user@host", tc.output)
			assert.Equal(t, tc.wantMessage, err.Message)
			assert.NotNil(t, err.Suggestion)
			if tc.wantHasDest {
				assert.Contains(t, err.Suggestion, "user@host")
			}
		})
	}
}

func TestClassifySSHErrorHostKeyContainsDest(t *testing.T) {
	err := classifySSHError("root@myserver.example.com", "Host key verification failed.")
	assert.Contains(t, err.Suggestion, "root@myserver.example.com")
}
