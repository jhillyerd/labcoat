package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// SSHCheckError describes a failure to connect to a remote host via SSH,
// providing a user-friendly diagnosis and suggested fix.
type SSHCheckError struct {
	// Dest is the SSH destination that was checked.
	Dest string
	// Message is a short description of the failure.
	Message string
	// Suggestion tells the user how to resolve the problem.
	Suggestion string
}

func (e *SSHCheckError) Error() string {
	return e.Message
}

// CheckSSH attempts a minimal SSH connection to verify that the target host is
// reachable, the host key is trusted, and authentication succeeds. Returns nil
// on success, or a *SSHCheckError describing the problem.
func CheckSSH(ctx context.Context, host string, user string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	dest := host
	if user != "" {
		dest = user + "@" + host
	}

	cmd := exec.CommandContext(ctx, "ssh", "-oBatchMode=yes", "-oConnectTimeout=5", dest, "true")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return nil
	}

	output := strings.TrimSpace(stderr.String())
	return classifySSHError(dest, output, err)
}

// classifySSHError maps raw SSH error output to a user-friendly SSHCheckError.
// Falls back to execErr when stderr is empty (e.g. ssh not found, context deadline).
func classifySSHError(dest string, output string, execErr error) *SSHCheckError {
	e := &SSHCheckError{Dest: dest}

	switch {
	case strings.Contains(output, "Host key verification failed"):
		e.Message = "Host key not trusted"
		e.Suggestion = fmt.Sprintf(
			"Run `ssh %s` from a terminal to accept the host key, then retry.", dest)

	case strings.Contains(output, "Permission denied"):
		e.Message = "SSH authentication failed"
		e.Suggestion = "Ensure your SSH key is loaded (ssh-add) and authorized on the target."

	case strings.Contains(output, "Connection refused"):
		e.Message = "Connection refused"
		e.Suggestion = "Ensure SSH is running on the target host."

	case strings.Contains(output, "Connection timed out"):
		e.Message = "Connection timed out"
		e.Suggestion = "Ensure the host is reachable on the network."

	case strings.Contains(output, "No route to host"):
		e.Message = "No route to host"
		e.Suggestion = "Ensure the host is reachable on the network."

	case strings.Contains(output, "Could not resolve hostname"):
		e.Message = "Could not resolve hostname"
		e.Suggestion = "Check that the hostname is correct and DNS is working."

	default:
		// execErr may indicate the ssh binary is missing, context was cancelled, etc.
		detail := output
		if detail == "" && execErr != nil {
			detail = execErr.Error()
		}

		if strings.Contains(detail, "executable file not found") {
			e.Message = "SSH command not found"
			e.Suggestion = "Ensure the `ssh` command is installed and in your PATH."
		} else if strings.Contains(detail, "context deadline") ||
			strings.Contains(detail, "signal: killed") {
			e.Message = "SSH check timed out"
			e.Suggestion = "The host may be unreachable. Check network connectivity and try again."
		} else {
			e.Message = "SSH connection failed"
			if detail != "" {
				e.Suggestion = detail
			} else {
				e.Suggestion = "Unknown error connecting via SSH."
			}
		}
	}

	return e
}
