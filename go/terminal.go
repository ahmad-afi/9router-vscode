package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\brm\s+-rf\s+/`),
	regexp.MustCompile(`(?i)\bmkfs\b`),
	regexp.MustCompile(`(?i)\bdd\s+if=`),
	regexp.MustCompile(`(?i):\(\)\s*\{.*\};\s*:`),
	regexp.MustCompile(`(?i)\bshutdown\b`),
	regexp.MustCompile(`(?i)\breboot\b`),
	regexp.MustCompile(`(?i)\bchmod\s+-R\s+777\s+/`),
}

func isDangerous(cmd string) bool {
	for _, p := range dangerousPatterns {
		if p.MatchString(cmd) {
			return true
		}
	}
	return false
}

func runTerminal(reqID int, command, workspacePath string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return writeResp(reqID, "error", ErrorParams{Message: "empty command"})
	}

	if isDangerous(command) {
		return writeResp(reqID, "error", ErrorParams{
			Message: "command blocked by safety filter: " + command,
		})
	}

	cmd := exec.Command("sh", "-c", command)
	if workspacePath != "" {
		cmd.Dir = workspacePath
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				return writeResp(reqID, "error", ErrorParams{Message: err.Error()})
			}
		}
		return writeResp(reqID, "tool.result", ToolResultParams{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: exitCode,
		})
	case <-time.After(30 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return writeResp(reqID, "error", ErrorParams{
			Message: "command timed out after 30s",
		})
	}
}
