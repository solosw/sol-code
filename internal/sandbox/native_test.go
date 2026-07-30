package sandbox

import (
	"context"
	"runtime"
	"testing"
)

func TestNewWithPolicyCreatesWorkspace(t *testing.T) {
	sb, err := NewWithPolicy(t.TempDir()+"/workspace", Policy{})
	if err != nil {
		t.Fatalf("NewWithPolicy: %v", err)
	}
	if sb.workDir == "" {
		t.Fatal("workspace path is empty")
	}
}

func TestRunDisabledExecutesCommand(t *testing.T) {
	program, args := "sh", []string{"-c", "printf sandbox-ok"}
	if runtime.GOOS == "windows" {
		program, args = "cmd", []string{"/c", "echo|set /p=sandbox-ok"}
	}

	sb, err := NewWithPolicy(t.TempDir(), Policy{})
	if err != nil {
		t.Fatalf("NewWithPolicy: %v", err)
	}
	result, err := sb.Run(context.Background(), Command{Program: program, Args: args})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Stdout != "sandbox-ok" {
		t.Fatalf("stdout = %q, want %q", result.Stdout, "sandbox-ok")
	}
}
