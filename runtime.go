package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const runtimeStatusTimeout = 1 * time.Second

const (
	runtimeURL = "http://127.0.0.1:8080"
	modelID    = "granite-4.1-8b-Q4_K_S.gguf"
)

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string  `json:"finish_reason"`
		Message      message `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Timings struct {
		PromptN            int     `json:"prompt_n"`
		PromptMS           float64 `json:"prompt_ms"`
		PromptPerSecond    float64 `json:"prompt_per_second"`
		PredictedN         int     `json:"predicted_n"`
		PredictedMS        float64 `json:"predicted_ms"`
		PredictedPerSecond float64 `json:"predicted_per_second"`
	} `json:"timings"`
}

type inferenceOutcome struct {
	Model               string
	Content             string
	FinishReason        string
	PromptTokens        int
	CompletionTokens    int
	TotalTokens         int
	PromptPerSecond     float64
	GenerationPerSecond float64
	Elapsed             time.Duration
}

type modelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
		Meta    struct {
			Context         int   `json:"n_ctx"`
			TrainingContext int   `json:"n_ctx_train"`
			Parameters      int64 `json:"n_params"`
			Size            int64 `json:"size"`
		} `json:"meta"`
	} `json:"data"`
}

func runtimeStatus(baseURL string, stdout, stderr io.Writer) int {
	client := &http.Client{
		Timeout: runtimeStatusTimeout,
	}

	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		fmt.Fprintf(stderr, "runtime unreachable: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintf(stderr, "runtime unhealthy: %s\n", resp.Status)
		return 1
	}

	fmt.Fprintf(stdout, "runtime: %s\n", resp.Status)
	return 0
}

func performInference(baseURL, requestedModel, prompt string) (inferenceOutcome, error) {
	payload := chatRequest{
		Model: requestedModel,
		Messages: []message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0,
		MaxTokens:   512,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return inferenceOutcome{}, fmt.Errorf("could not encode request: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		baseURL+"/v1/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return inferenceOutcome{}, fmt.Errorf("could not create inference request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	startedAt := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return inferenceOutcome{}, fmt.Errorf("inference request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return inferenceOutcome{}, fmt.Errorf("inference failed: HTTP %s", resp.Status)
	}

	var result chatResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return inferenceOutcome{}, fmt.Errorf("could not decode inference response: %w", err)
	}

	if len(result.Choices) == 0 {
		return inferenceOutcome{}, fmt.Errorf("inference response contained no choices")
	}

	choice := result.Choices[0]

	return inferenceOutcome{
		Model:               result.Model,
		Content:             choice.Message.Content,
		FinishReason:        choice.FinishReason,
		PromptTokens:        result.Usage.PromptTokens,
		CompletionTokens:    result.Usage.CompletionTokens,
		TotalTokens:         result.Usage.TotalTokens,
		PromptPerSecond:     result.Timings.PromptPerSecond,
		GenerationPerSecond: result.Timings.PredictedPerSecond,
		Elapsed:             time.Since(startedAt),
	}, nil
}

func runtimeInfer(baseURL, prompt string, stdout, stderr io.Writer) int {
	outcome, err := performInference(baseURL, inferenceModelID(baseURL), prompt)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	fmt.Fprintln(stdout, outcome.Content)
	fmt.Fprintf(stderr, "finish_reason: %s\n", outcome.FinishReason)

	return 0
}

func runtimeInspect(baseURL string, stdout, stderr io.Writer) int {
	resp, err := http.Get(baseURL + "/v1/models")
	if err != nil {
		fmt.Fprintf(stderr, "runtime inspection failed: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintf(stderr, "runtime inspection failed: HTTP %s\n", resp.Status)
		return 1
	}

	var result modelsResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stderr, "could not decode runtime inspection response: %v\n", err)
		return 1
	}

	if len(result.Data) == 0 {
		fmt.Fprintln(stderr, "runtime inspection returned no models")
		return 1
	}

	model := result.Data[0]

	fmt.Fprintf(stdout, "runtime: %s\n", model.OwnedBy)
	fmt.Fprintf(stdout, "model: %s\n", model.ID)
	fmt.Fprintf(stdout, "context: %d\n", model.Meta.Context)
	fmt.Fprintf(stdout, "training context: %d\n", model.Meta.TrainingContext)
	fmt.Fprintf(stdout, "parameters: %d\n", model.Meta.Parameters)
	fmt.Fprintf(stdout, "size: %d\n", model.Meta.Size)

	return 0
}
