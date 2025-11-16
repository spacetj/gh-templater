package runner

import (
	"bytes"
	"fmt"
	"os/exec"
)

// Runner executes commands and returns stdout or an error containing stderr.
type Runner interface {
	Run(cmd string, args ...string) (string, error)
}

// ExecRunner runs commands on the host system.
type ExecRunner struct{}

// Run executes the given command with arguments and returns stdout.
func (ExecRunner) Run(cmd string, args ...string) (string, error) {
	command := exec.Command(cmd, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
