package utils

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// ExecuteCommand executes a shell command and returns an error if it fails
func ExecuteCommand(command string) error {
	Info.Printf("Executing command: %s", command)
	var cmd *exec.Cmd
	if isWindows() {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		Error.Printf("Command failed: %s", string(output))
		return fmt.Errorf("command failed: %v, output: %s", err, string(output))
	}

	Info.Printf("Command output: %s", string(output))
	return nil
}

func isWindows() bool {
	return strings.Contains(strings.ToLower(runtime.GOOS), "windows")
}
