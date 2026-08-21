package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	llamaServerPath      = "/opt/homebrew/bin/llama-server"
	runtimeStartupLimit  = 30 * time.Second
	runtimeShutdownLimit = 5 * time.Second
)

type runtimeState struct {
	PID         int       `json:"pid"`
	Executable  string    `json:"executable"`
	Model       string    `json:"model"`
	ModelID     string    `json:"model_id,omitempty"`
	URL         string    `json:"url"`
	StartedAt   time.Time `json:"started_at"`
	ProfileID   string    `json:"profile_id,omitempty"`
	Context     int       `json:"context,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

func defaultModelPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(
		home,
		".lmstudio",
		"models",
		"ibm-granite",
		"granite-4.1-8b-GGUF",
		"granite-4.1-8b-Q4_K_S.gguf",
	), nil
}

func runtimeStart(stdout, stderr io.Writer) int {
	modelPath, err := defaultModelPath()
	if err != nil {
		fmt.Fprintf(stderr, "could not determine default model path: %v\n", err)
		return 1
	}

	return runtimeStartModel(modelPath, stdout, stderr)
}

func runtimeStartModel(modelPath string, stdout, stderr io.Writer) int {
	return runtimeStartModelWithProfile(modelPath, defaultProfile(), stdout, stderr)
}

func runtimeStartModelWithProfile(modelPath string, profile profileDefinition, stdout, stderr io.Writer) int {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "could not determine home directory: %v\n", err)
		return 1
	}

	if _, err := os.Stat(llamaServerPath); err != nil {
		fmt.Fprintf(stderr, "llama-server not available: %v\n", err)
		return 1
	}

	if _, err := os.Stat(modelPath); err != nil {
		fmt.Fprintf(stderr, "model not available: %v\n", err)
		return 1
	}
	if profile.Context <= 0 {
		profile = defaultProfile()
	}

	stateDir := filepath.Join(home, ".localctl")

	if err := os.MkdirAll(stateDir, 0700); err != nil {
		fmt.Fprintf(stderr, "could not create state directory: %v\n", err)
		return 1
	}

	statePath := filepath.Join(stateDir, "runtime.json")
	logPath := filepath.Join(stateDir, "llama-server.log")

	if _, err := os.Stat(statePath); err == nil {
		fmt.Fprintln(stderr, "runtime state already exists; stop the managed runtime first")
		return 1
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "could not inspect runtime state: %v\n", err)
		return 1
	}

	var statusOut bytes.Buffer
	var statusErr bytes.Buffer

	if runtimeStatus(runtimeURL, &statusOut, &statusErr) == 0 {
		fmt.Fprintf(stderr, "a runtime is already responding at %s but is not managed by localctl\n", runtimeURL)
		return 1
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Fprintf(stderr, "could not open runtime log: %v\n", err)
		return 1
	}

	cmd := exec.Command(
		llamaServerPath,
		"--model", modelPath,
		"--host", "127.0.0.1",
		"--port", "8080",
		"--ctx-size", strconv.Itoa(profile.Context),
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	startedAt := time.Now()
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		fmt.Fprintf(stderr, "could not start llama-server: %v\n", err)
		return 1
	}
	_ = logFile.Close()

	state := runtimeState{
		PID:         cmd.Process.Pid,
		Executable:  llamaServerPath,
		Model:       modelPath,
		ModelID:     filepath.Base(modelPath),
		URL:         runtimeURL,
		StartedAt:   startedAt,
		ProfileID:   profile.ID,
		Context:     profile.Context,
		Temperature: profile.Temperature,
		MaxTokens:   profile.MaxTokens,
	}

	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		fmt.Fprintf(stderr, "could not encode runtime state: %v\n", err)
		return 1
	}
	if err := os.WriteFile(statePath, stateJSON, 0600); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		fmt.Fprintf(stderr, "could not record runtime state: %v\n", err)
		return 1
	}

	deadline := time.Now().Add(runtimeStartupLimit)
	for {
		var healthOut bytes.Buffer
		var healthErr bytes.Buffer
		if runtimeStatus(runtimeURL, &healthOut, &healthErr) == 0 {
			fmt.Fprintln(stdout, "runtime started")
			fmt.Fprintf(stdout, "pid: %d\n", cmd.Process.Pid)
			fmt.Fprintf(stdout, "model: %s\n", state.ModelID)
			fmt.Fprintf(stdout, "profile: %s (ctx=%d, temp=%g, max=%d)\n", profile.ID, profile.Context, profile.Temperature, profile.MaxTokens)
			fmt.Fprintf(stdout, "url: %s\n", runtimeURL)
			fmt.Fprintf(stdout, "startup: %s\n", time.Since(startedAt).Round(time.Millisecond))
			fmt.Fprintf(stdout, "log: %s\n", logPath)
			return 0
		}
		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			_ = cmd.Wait()
			_ = os.Remove(statePath)
			fmt.Fprintf(stderr, "llama-server exited before becoming ready; see %s\n", logPath)
			return 1
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			_ = cmd.Wait()
			_ = os.Remove(statePath)
			fmt.Fprintf(stderr, "llama-server did not become ready within %s; see %s\n", runtimeStartupLimit, logPath)
			return 1
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func runtimeStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".localctl", "runtime.json"), nil
}

func readRuntimeState() (runtimeState, error) {
	path, err := runtimeStatePath()
	if err != nil {
		return runtimeState{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return runtimeState{}, err
	}
	var state runtimeState
	if err := json.Unmarshal(data, &state); err != nil {
		return runtimeState{}, err
	}
	if state.ModelID == "" {
		state.ModelID = filepath.Base(state.Model)
	}
	if state.ProfileID == "" {
		state.ProfileID = "default"
	}
	if state.Context == 0 {
		state.Context = defaultProfile().Context
	}
	if state.MaxTokens == 0 {
		state.MaxTokens = defaultProfile().MaxTokens
	}
	return state, nil
}

func inferenceModelID(baseURL string) string {
	if baseURL != runtimeURL {
		return modelID
	}
	state, err := readRuntimeState()
	if err != nil || state.URL != baseURL || state.ModelID == "" {
		return modelID
	}
	return state.ModelID
}

func runtimeStop(stdout, stderr io.Writer) int {
	statePath, err := runtimeStatePath()
	if err != nil {
		fmt.Fprintf(stderr, "could not determine runtime state path: %v\n", err)
		return 1
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(stderr, "no managed runtime state found")
			return 1
		}
		fmt.Fprintf(stderr, "could not read runtime state: %v\n", err)
		return 1
	}
	var state runtimeState
	if err := json.Unmarshal(data, &state); err != nil {
		fmt.Fprintf(stderr, "could not decode runtime state: %v\n", err)
		return 1
	}
	process, err := os.FindProcess(state.PID)
	if err != nil {
		fmt.Fprintf(stderr, "could not find runtime process: %v\n", err)
		return 1
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		_ = os.Remove(statePath)
		fmt.Fprintf(stderr, "recorded runtime PID %d is no longer running; removed stale state\n", state.PID)
		return 1
	}
	psOutput, err := exec.Command("/bin/ps", "-p", strconv.Itoa(state.PID), "-o", "command=").Output()
	if err != nil {
		fmt.Fprintf(stderr, "could not verify runtime process identity: %v\n", err)
		return 1
	}
	commandLine := string(psOutput)
	if !strings.Contains(commandLine, state.Executable) || !strings.Contains(commandLine, state.Model) {
		fmt.Fprintf(stderr, "refusing to signal PID %d because its process identity does not match recorded runtime state\n", state.PID)
		return 1
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(stderr, "could not send SIGTERM to runtime: %v\n", err)
		return 1
	}
	deadline := time.Now().Add(runtimeShutdownLimit)
	for {
		err := process.Signal(syscall.Signal(0))
		if err != nil {
			_ = os.Remove(statePath)
			fmt.Fprintln(stdout, "runtime stopped")
			fmt.Fprintf(stdout, "pid: %d\n", state.PID)
			return 0
		}
		if time.Now().After(deadline) {
			fmt.Fprintf(stderr, "runtime did not stop within %s\n", runtimeShutdownLimit)
			return 1
		}
		time.Sleep(100 * time.Millisecond)
	}
}
