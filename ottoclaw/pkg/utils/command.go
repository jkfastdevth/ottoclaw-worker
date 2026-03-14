package utils

import (
	"os/exec"
	"strings"
)

// RunCommand executes a shell command and returns its combined output (stdout and stderr).
func RunCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
