package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantExit   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "version",
			args:       []string{"localctl", "version"},
			wantExit:   0,
			wantStdout: "localctl 0.5.0\n",
			wantStderr: "",
		},
		{
			name:       "unknown command",
			args:       []string{"localctl", "garbage"},
			wantExit:   1,
			wantStdout: "",
			wantStderr: "unknown command: garbage\nRun 'localctl help' to see the learner-facing command map.\n",
		},
		{
			name:       "capability missing model",
			args:       []string{"localctl", "capability"},
			wantExit:   1,
			wantStdout: "",
			wantStderr: "usage: localctl capability <model> [--profile=default] [--json]\n",
		},
		{
			name:       "recommend missing pack",
			args:       []string{"localctl", "recommend"},
			wantExit:   1,
			wantStdout: "",
			wantStderr: "usage: localctl recommend <pack> [--profile=default] [--json]\n",
		},
		{
			name:       "requalify missing model",
			args:       []string{"localctl", "requalify"},
			wantExit:   1,
			wantStdout: "",
			wantStderr: "usage: localctl requalify <model> [--profile=default] [--run]\n",
		},
		{
			name:       "runtime missing subcommand",
			args:       []string{"localctl", "runtime"},
			wantExit:   1,
			wantStdout: "",
			wantStderr: "usage: localctl runtime <start|stop|status|inspect|infer>\n",
		},
		{
			name:       "runtime unknown subcommand",
			args:       []string{"localctl", "runtime", "garbage"},
			wantExit:   1,
			wantStdout: "",
			wantStderr: "unknown runtime command: garbage\n",
		},
		{
			name:       "runtime infer missing prompt",
			args:       []string{"localctl", "runtime", "infer"},
			wantExit:   1,
			wantStdout: "",
			wantStderr: "usage: localctl runtime infer <prompt>\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := run(tt.args, &stdout, &stderr)
			if exitCode != tt.wantExit {
				t.Fatalf("expected exit %d, got %d", tt.wantExit, exitCode)
			}
			if stdout.String() != tt.wantStdout {
				t.Fatalf("expected stdout %q, got %q", tt.wantStdout, stdout.String())
			}
			if stderr.String() != tt.wantStderr {
				t.Fatalf("expected stderr %q, got %q", tt.wantStderr, stderr.String())
			}
		})
	}
}

func TestRootCLIShowsLearnerHelp(t *testing.T) {
	for _, args := range [][]string{{"localctl"}, {"localctl", "help"}, {"localctl", "--help"}} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("help path %v returned %d: %s", args, code, stderr.String())
		}
		for _, want := range []string{"Start", "localctl lab", "Compare and learn", "Read the evidence", "Glass box", "localctl capability", "localctl recommend"} {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("help path %v missing %q:\n%s", args, want, stdout.String())
			}
		}
	}
}

func TestRuntimeInferJoinsPromptArguments(t *testing.T) {
	if !strings.Contains("runtime infer <prompt>", "<prompt>") {
		t.Fatal("sanity check failed")
	}
}
