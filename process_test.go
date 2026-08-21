package main

import (
	"bytes"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestChildProcessLifecycle(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command(
		"/bin/sh",
		"-c",
		`echo CHILD_STARTED; sleep 1; echo CHILD_FINISHED`,
	)

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if cmd.Process != nil {
		t.Fatalf("expected no process before Start")
	}

	start := time.Now()

	if err := cmd.Start(); err != nil {
		t.Fatalf("could not start child: %v", err)
	}

	startElapsed := time.Since(start)

	t.Logf("child PID: %d", cmd.Process.Pid)
	t.Logf("Start returned after: %s", startElapsed)

	if cmd.Process.Pid <= 0 {
		t.Fatalf("expected a real child PID")
	}

	if startElapsed >= 500*time.Millisecond {
		t.Fatalf(
			"expected Start to return before child completed, took %s",
			startElapsed,
		)
	}

	if err := cmd.Wait(); err != nil {
		t.Fatalf("child process failed: %v", err)
	}

	totalElapsed := time.Since(start)

	t.Logf("Wait completed after: %s", totalElapsed)
	t.Logf("stdout: %q", stdout.String())
	t.Logf("stderr: %q", stderr.String())

	if !cmd.ProcessState.Exited() {
		t.Fatalf("expected child process to have exited normally")
	}

	if !cmd.ProcessState.Success() {
		t.Fatalf("expected child process to succeed")
	}

	if stdout.String() != "CHILD_STARTED\nCHILD_FINISHED\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestChildProcessTermination(t *testing.T) {
	cmd := exec.Command(
		"/bin/sh",
		"-c",
		`sleep 10`,
	)

	if err := cmd.Start(); err != nil {
		t.Fatalf("could not start child: %v", err)
	}

	t.Logf("child PID: %d", cmd.Process.Pid)

	if cmd.ProcessState != nil {
		t.Fatalf("expected no ProcessState before Wait")
	}

	start := time.Now()

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("could not kill child: %v", err)
	}

	err := cmd.Wait()
	elapsed := time.Since(start)

	t.Logf("Wait returned after kill in: %s", elapsed)
	t.Logf("Wait error: %v", err)

	if err == nil {
		t.Fatalf("expected Wait to report terminated child")
	}

	if cmd.ProcessState == nil {
		t.Fatalf("expected ProcessState after Wait")
	}

	if cmd.ProcessState.Success() {
		t.Fatalf("expected killed child to be unsuccessful")
	}
}

func TestChildProcessGracefulTermination(t *testing.T) {
	cmd := exec.Command(
		"/bin/sh",
		"-c",
		`trap 'exit 0' TERM; while true; do sleep 1; done`,
	)

	if err := cmd.Start(); err != nil {
		t.Fatalf("could not start child: %v", err)
	}

	t.Logf("child PID: %d", cmd.Process.Pid)

	time.Sleep(100 * time.Millisecond)

	start := time.Now()

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("could not signal child: %v", err)
	}

	err := cmd.Wait()
	elapsed := time.Since(start)

	t.Logf("Wait returned after SIGTERM in: %s", elapsed)
	t.Logf("Wait error: %v", err)

	if err != nil {
		t.Fatalf("expected graceful child shutdown, got: %v", err)
	}

	if cmd.ProcessState == nil {
		t.Fatalf("expected ProcessState after Wait")
	}

	if !cmd.ProcessState.Success() {
		t.Fatalf("expected graceful shutdown to succeed")
	}
}
