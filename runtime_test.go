package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRuntimeStatusHealthy(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}

			if r.URL.Path != "/health" {
				t.Errorf("expected /health, got %s", r.URL.Path)
			}

			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runtimeStatus(
		server.URL,
		&stdout,
		&stderr,
	)

	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}

	if stdout.String() != "runtime: 200 OK\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	if stderr.String() != "" {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRuntimeStatusUnhealthy(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}),
	)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runtimeStatus(
		server.URL,
		&stdout,
		&stderr,
	)

	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode)
	}

	if stdout.String() != "" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	if stderr.String() != "runtime unhealthy: 503 Service Unavailable\n" {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}
func TestRuntimeStatusUnreachable(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runtimeStatus(
		"http://127.0.0.1:1",
		&stdout,
		&stderr,
	)

	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode)
	}

	if stdout.String() != "" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	if !strings.Contains(stderr.String(), "runtime unreachable:") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRuntimeInferSuccess(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}

			if r.URL.Path != "/v1/chat/completions" {
				t.Errorf(
					"expected /v1/chat/completions, got %s",
					r.URL.Path,
				)
			}

			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf(
					"expected Content-Type application/json, got %q",
					r.Header.Get("Content-Type"),
				)
			}

			var request chatRequest

			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("could not decode request: %v", err)
			}

			if request.Model != modelID {
				t.Errorf(
					"expected model %q, got %q",
					modelID,
					request.Model,
				)
			}

			if request.Temperature != 0 {
				t.Errorf(
					"expected temperature 0, got %v",
					request.Temperature,
				)
			}

			if request.MaxTokens != 32 {
				t.Errorf(
					"expected max tokens 32, got %d",
					request.MaxTokens,
				)
			}

			if len(request.Messages) != 1 {
				t.Fatalf(
					"expected 1 message, got %d",
					len(request.Messages),
				)
			}

			if request.Messages[0].Role != "user" {
				t.Errorf(
					"expected role %q, got %q",
					"user",
					request.Messages[0].Role,
				)
			}

			if request.Messages[0].Content != "hello" {
				t.Errorf(
					"expected prompt %q, got %q",
					"hello",
					request.Messages[0].Content,
				)
			}

			w.Header().Set("Content-Type", "application/json")

			w.Write([]byte(`{
				"model": "fake-model",
				"choices": [
					{
						"finish_reason": "stop",
						"message": {
							"role": "assistant",
							"content": "LOCALCTL_TEST_OK"
						}
					}
				]
			}`))
		}),
	)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runtimeInfer(
		server.URL,
		"hello",
		&stdout,
		&stderr,
	)

	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}

	if stdout.String() != "LOCALCTL_TEST_OK\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	if stderr.String() != "" {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}
func TestRuntimeInferMalformedJSON(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`this is not json`))
		}),
	)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runtimeInfer(
		server.URL,
		"hello",
		&stdout,
		&stderr,
	)

	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode)
	}

	if stdout.String() != "" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	if !strings.Contains(
		stderr.String(),
		"could not decode inference response:",
	) {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRuntimeInferNoChoices(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			w.Write([]byte(`{
				"model": "fake-model",
				"choices": []
			}`))
		}),
	)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runtimeInfer(
		server.URL,
		"hello",
		&stdout,
		&stderr,
	)

	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode)
	}

	if stdout.String() != "" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	if stderr.String() != "inference response contained no choices\n" {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRuntimeInferHTTPFailure(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}),
	)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runtimeInfer(
		server.URL,
		"hello",
		&stdout,
		&stderr,
	)

	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode)
	}

	if stdout.String() != "" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	if stderr.String() != "inference failed: HTTP 503 Service Unavailable\n" {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}
