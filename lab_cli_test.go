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

func TestDecideLabRuntime(t *testing.T) {
	model := modelArtifact{Path: "/models/granite.gguf"}

	tests := []struct {
		name  string
		state *runtimeState
		ready bool
		want  labRuntimeAction
	}{
		{
			name: "start without state",
			want: labRuntimeStart,
		},
		{
			name:  "reuse same ready model",
			state: &runtimeState{Model: "/models/granite.gguf"},
			ready: true,
			want:  labRuntimeReuse,
		},
		{
			name:  "switch different ready model",
			state: &runtimeState{Model: "/models/ornith.gguf"},
			ready: true,
			want:  labRuntimeSwitch,
		},
		{
			name:  "reconcile state that is not ready",
			state: &runtimeState{Model: "/models/granite.gguf"},
			ready: false,
			want:  labRuntimeReconcile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideLabRuntime(tt.state, tt.ready, model)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestBaselineOneLine(t *testing.T) {
	got := baselineOneLine("one\n  two\tthree", 80)
	if got != "one two three" {
		t.Fatalf("unexpected normalized detail: %q", got)
	}
}
