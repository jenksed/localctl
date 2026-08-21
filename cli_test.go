package main

import (
	"bytes"
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
			wantStdout: "localctl dev\n",
			wantStderr: "",
		},
		{
			name:       "missing command",
			args:       []string{"localctl"},
			wantExit:   1,
			wantStdout: "",
			wantStderr: "usage: localctl <command>\n",
		},
		{
			name:       "unknown command",
			args:       []string{"localctl", "garbage"},
			wantExit:   1,
			wantStdout: "",
			wantStderr: "unknown command: garbage\n",
		},
		{
			name:       "runtime missing subcommand",
			args:       []string{"localctl", "runtime"},
			wantExit:   1,
			wantStdout: "",
			wantStderr: "usage: localctl runtime <status|infer>\n",
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

			exitCode := run(
				tt.args,
				&stdout,
				&stderr,
			)

			if exitCode != tt.wantExit {
				t.Fatalf(
					"expected exit %d, got %d",
					tt.wantExit,
					exitCode,
				)
			}

			if stdout.String() != tt.wantStdout {
				t.Fatalf(
					"expected stdout %q, got %q",
					tt.wantStdout,
					stdout.String(),
				)
			}

			if stderr.String() != tt.wantStderr {
				t.Fatalf(
					"expected stderr %q, got %q",
					tt.wantStderr,
					stderr.String(),
				)
			}
		})
	}
}
