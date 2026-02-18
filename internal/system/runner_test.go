package system

import (
	"strings"
	"testing"
)

func TestBuildShellWrapper_NoEnv_NoRoot(t *testing.T) {
	cmd := NewCommand("echo", "hello", "world")

	argv, err := cmd.BuildShellWrapper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(argv) != 3 {
		t.Fatalf("expected 3 args (sh -c script), got %d: %#v", len(argv), argv)
	}

	if argv[0] != "sh" || argv[1] != "-c" {
		t.Fatalf("expected sh -c prefix, got: %#v", argv[:2])
	}

	script := argv[2]

	if !strings.Contains(script, `set --`) {
		t.Fatalf("expected script to contain 'set --', got: %s", script)
	}

	if !strings.Contains(script, `exec "$@"`) {
		t.Fatalf("expected script to exec \"$@\", got: %s", script)
	}

	if !strings.Contains(script, "echo") {
		t.Fatalf("expected script to contain command name, got: %s", script)
	}
}

func TestBuildShellWrapper_WithEnv_DeterministicOrder(t *testing.T) {
	cmd := NewCommand("true")
	cmd.SetEnv("B", "2")
	cmd.SetEnv("A", "1")

	argv, err := cmd.BuildShellWrapper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	script := argv[2]

	aIndex := strings.Index(script, "export A=")
	bIndex := strings.Index(script, "export B=")

	if aIndex == -1 || bIndex == -1 {
		t.Fatalf("expected exports for A and B, got: %s", script)
	}

	if aIndex > bIndex {
		t.Fatalf("expected env vars sorted alphabetically, got: %s", script)
	}
}

func TestBuildShellWrapper_InvalidEnvName(t *testing.T) {
	cmd := NewCommand("true")
	cmd.SetEnv("INVALID-NAME", "value")

	_, err := cmd.BuildShellWrapper()
	if err == nil {
		t.Fatalf("expected error for invalid env name")
	}

	if !strings.Contains(err.Error(), "invalid env var name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildShellWrapper_NulInEnvValue(t *testing.T) {
	cmd := NewCommand("true")
	cmd.SetEnv("VALID", "bad\x00value")

	_, err := cmd.BuildShellWrapper()
	if err == nil {
		t.Fatalf("expected error for NUL byte in env value")
	}
}

func TestBuildShellWrapper_NulInArgs(t *testing.T) {
	cmd := NewCommand("echo", "bad\x00arg")

	_, err := cmd.BuildShellWrapper()
	if err == nil {
		t.Fatalf("expected error for NUL byte in arg")
	}
}

func TestBuildShellWrapper_WithRootElevation(t *testing.T) {
	cmd := NewCommand("id")
	cmd.AsRoot("sudo", "-n")

	argv, err := cmd.BuildShellWrapper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(argv) < 4 {
		t.Fatalf("expected root elevation prefix, got: %#v", argv)
	}

	if argv[0] != "sudo" || argv[1] != "-n" {
		t.Fatalf("expected sudo -n prefix, got: %#v", argv[:2])
	}

	if argv[2] != "sh" || argv[3] != "-c" {
		t.Fatalf("expected sh -c after root prefix, got: %#v", argv)
	}
}

func TestBuildShellWrapper_EnvAndArgsProperlyQuoted(t *testing.T) {
	cmd := NewCommand("echo", `hello world`, `foo"bar`)
	cmd.SetEnv("TEST", `value with spaces`)

	argv, err := cmd.BuildShellWrapper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	script := argv[2]

	if !strings.Contains(script, "export TEST=") {
		t.Fatalf("missing export statement: %s", script)
	}

	if !strings.Contains(script, "hello world") {
		t.Fatalf("expected quoted arg with space present: %s", script)
	}
}
