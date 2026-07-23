package utils

import (
	"strings"
	"testing"
)

func TestIsWindows(t *testing.T) {
	cases := []struct {
		goos string
		want bool
	}{
		{"windows", true},
		{"Windows", true},
		{"WINDOWS", true},
		{"linux", false},
		{"darwin", false},
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.goos, func(t *testing.T) {
			if got := isWindows(tc.goos); got != tc.want {
				t.Errorf("isWindows(%q) = %v, want %v", tc.goos, got, tc.want)
			}
		})
	}
}

func TestExecuteCommand_Success(t *testing.T) {
	if err := ExecuteCommand("true"); err != nil {
		t.Errorf("expected nil error for a command that exits 0, got %v", err)
	}
}

func TestExecuteCommand_ReturnsErrorOnNonZeroExit(t *testing.T) {
	err := ExecuteCommand("exit 1")
	if err == nil {
		t.Error("expected an error for a command that exits non-zero, got nil")
	}
}

func TestExecuteCommand_IncludesOutputOnFailure(t *testing.T) {
	err := ExecuteCommand("echo failure-marker && exit 1")
	if err == nil {
		t.Fatal("expected an error for a command that exits non-zero, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "failure-marker") {
		t.Errorf("expected error to include the command's output, got %q", got)
	}
}
