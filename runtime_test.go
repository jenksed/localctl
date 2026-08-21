package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

			if request.MaxTokens != 512 {
				t.Errorf(
					"expected max tokens 512, got %d",
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

	if stderr.String() != "finish_reason: stop\n" {
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

func TestRuntimeInspectSuccess(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}

			if r.URL.Path != "/v1/models" {
				t.Errorf(
					"expected /v1/models, got %s",
					r.URL.Path,
				)
			}

			w.Header().Set("Content-Type", "application/json")

			w.Write([]byte(`{
				"data": [
					{
						"id": "test-model.gguf",
						"owned_by": "llamacpp",
						"meta": {
							"n_ctx": 2048,
							"n_ctx_train": 131072,
							"n_params": 8791592960,
							"size": 5087248384
						}
					}
				]
			}`))
		}),
	)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runtimeInspect(
		server.URL,
		&stdout,
		&stderr,
	)

	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}

	expectedOutput := `runtime: llamacpp
model: test-model.gguf
context: 2048
training context: 131072
parameters: 8791592960
size: 5087248384
`

	if stdout.String() != expectedOutput {
		t.Fatalf(
			"unexpected stdout:\nexpected:\n%s\ngot:\n%s",
			expectedOutput,
			stdout.String(),
		)
	}

	if stderr.String() != "" {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRuntimeInspectMalformedJSON(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`this is not json`))
		}),
	)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runtimeInspect(
		server.URL,
		&stdout,
		&stderr,
	)

	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode)
	}

	if !strings.Contains(
		stderr.String(),
		"could not decode runtime inspection response:",
	) {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRuntimeInspectNoModels(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			w.Write([]byte(`{
				"data": []
			}`))
		}),
	)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runtimeInspect(
		server.URL,
		&stdout,
		&stderr,
	)

	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode)
	}

	if stderr.String() != "runtime inspection returned no models\n" {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRuntimeStatusSlowServer(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(2 * time.Second):
				w.WriteHeader(http.StatusOK)

			case <-r.Context().Done():
				return
			}
		}),
	)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	start := time.Now()

	exitCode := runtimeStatus(
		server.URL,
		&stdout,
		&stderr,
	)

	elapsed := time.Since(start)

	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode)
	}

	if elapsed >= 1500*time.Millisecond {
		t.Fatalf(
			"expected runtime status to time out before 1.5s, took %s",
			elapsed,
		)
	}

	if stdout.String() != "" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	if !strings.Contains(stderr.String(), "runtime unreachable:") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}
