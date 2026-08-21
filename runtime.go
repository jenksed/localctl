package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

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
}

func runtimeStatus(baseURL string, stdout, stderr io.Writer) int {
	resp, err := http.Get(baseURL + "/health")
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

func runtimeInfer(baseURL, prompt string, stdout, stderr io.Writer) int {
	payload := chatRequest{
		Model: modelID,
		Messages: []message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0,
		MaxTokens:   32,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(stderr, "could not encode request: %v\n", err)
		return 1
	}

	req, err := http.NewRequest(
		http.MethodPost,
		baseURL+"/v1/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		fmt.Fprintf(stderr, "could not create inference request: %v\n", err)
		return 1
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "inference request failed: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintf(stderr, "inference failed: HTTP %s\n", resp.Status)
		return 1
	}

	var result chatResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stderr, "could not decode inference response: %v\n", err)
		return 1
	}

	if len(result.Choices) == 0 {
		fmt.Fprintln(stderr, "inference response contained no choices")
		return 1
	}

	fmt.Fprintln(stdout, result.Choices[0].Message.Content)
	return 0
}
