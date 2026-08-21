package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLlamaServerChildProcess(t *testing.T) {
	if os.Getenv("LOCALCTL_INTEGRATION") != "1" {
		t.Skip("set LOCALCTL_INTEGRATION=1 to run real llama-server integration test")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not determine home directory: %v", err)
	}

	modelPath := filepath.Join(
		home,
		".lmstudio",
		"models",
		"ibm-granite",
		"granite-4.1-8b-GGUF",
		"granite-4.1-8b-Q4_K_S.gguf",
	)

	if _, err := os.Stat(modelPath); err != nil {
		t.Fatalf("model not available at %s: %v", modelPath, err)
	}

	const port = "18080"
	baseURL := "http://127.0.0.1:" + port

	var logs bytes.Buffer

	cmd := exec.Command(
		"/opt/homebrew/bin/llama-server",
		"--model", modelPath,
		"--host", "127.0.0.1",
		"--port", port,
		"--ctx-size", "2048",
	)

	cmd.Stdout = &logs
	cmd.Stderr = &logs

	start := time.Now()

	if err := cmd.Start(); err != nil {
		t.Fatalf("could not start llama-server: %v", err)
	}

	t.Logf("llama-server PID: %d", cmd.Process.Pid)
	t.Logf("Start returned after: %s", time.Since(start))

	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()

	deadline := time.Now().Add(30 * time.Second)

	for {
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		exitCode := runtimeStatus(
			baseURL,
			&stdout,
			&stderr,
		)

		if exitCode == 0 {
			t.Logf("runtime became ready after: %s", time.Since(start))
			t.Logf("status output: %q", stdout.String())
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf(
				"llama-server did not become ready\nlogs:\n%s",
				logs.String(),
			)
		}

		time.Sleep(250 * time.Millisecond)
	}

	var inspectOut bytes.Buffer
	var inspectErr bytes.Buffer

	if exitCode := runtimeInspect(
		baseURL,
		&inspectOut,
		&inspectErr,
	); exitCode != 0 {
		t.Fatalf(
			"runtime inspection failed: %s\nlogs:\n%s",
			inspectErr.String(),
			logs.String(),
		)
	}

	t.Logf("inspection:\n%s", inspectOut.String())

	if !strings.Contains(
		inspectOut.String(),
		"model: granite-4.1-8b-Q4_K_S.gguf",
	) {
		t.Fatalf(
			"expected Granite model in inspection, got:\n%s",
			inspectOut.String(),
		)
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("could not stop llama-server: %v", err)
	}

	if err := cmd.Wait(); err == nil {
		t.Fatalf("expected killed llama-server to return an error")
	} else {
		t.Logf("llama-server termination: %v", err)
	}

	if cmd.ProcessState == nil {
		t.Fatalf("expected ProcessState after Wait")
	}

	t.Logf(
		"llama-server lifetime: %s",
		time.Since(start),
	)

	if logs.Len() == 0 {
		t.Fatalf("expected llama-server logs")
	}

	fmt.Fprintf(
		os.Stderr,
		"\n===== LLAMA-SERVER LOG TAIL =====\n%s\n",
		lastLines(logs.String(), 20),
	)
}

func lastLines(text string, count int) string {
	lines := strings.Split(text, "\n")

	if len(lines) <= count {
		return text
	}

	return strings.Join(lines[len(lines)-count:], "\n")
}
