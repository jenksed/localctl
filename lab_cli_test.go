package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestExercisesCLI(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{"localctl", "exercises"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "exact-output-v1") {
		t.Fatalf("expected core exercise listing, got: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "go-review-pid-race-v1") {
		t.Fatalf("extended exercise should not appear in core listing")
	}
}

func TestExercisesCLIAll(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{"localctl", "exercises", "--all"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "go-review-pid-race-v1") {
		t.Fatalf("expected extended exercise listing, got: %s", stdout.String())
	}
}

func TestExerciseShow(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{"localctl", "exercise", "show", "go-slice-alias-v1"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Go slice aliasing") || !strings.Contains(stdout.String(), "a := []int{1, 2, 3}") {
		t.Fatalf("unexpected exercise output: %s", stdout.String())
	}
}

func TestExerciseUnknown(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{"localctl", "exercise", "show", "not-real"},
		&stdout,
		&stderr,
	)
	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode)
	}
	if stderr.String() != "unknown exercise: not-real\n" {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}
