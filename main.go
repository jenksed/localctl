package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

func main() {
	if len(os.Args) < 2 {
		// print usage
		// exit non-zero
		fmt.Fprintln(os.Stderr, "usage: localctl <command>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version":
		fmt.Println("localctl dev")

	case "runtime":
		// validate os.Args[2] exists
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: localctl runtime <status|infer>")
			os.Exit(1)
		}

		switch os.Args[2] {
		case "status":
			resp, err := http.Get("http://127.0.0.1:8080/health")
			if err != nil {
				fmt.Fprintf(os.Stderr, "runtime unreachable: %v\n", err)
				os.Exit(1)
			}

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				fmt.Fprintf(os.Stderr, "runtime unhealthy: %s\n", resp.Status)
				resp.Body.Close()
				os.Exit(1)
			}

			fmt.Printf("runtime: %s\n", resp.Status)
			resp.Body.Close()

		case "infer":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "usage: localctl runtime infer <prompt>")
				os.Exit(1)
			}

			payload := chatRequest{
				Model: "granite-4.1-8b-Q4_K_S.gguf",
				Messages: []message{
					{
						Role:    "user",
						Content: os.Args[3],
					},
				},
				Temperature: 0,
				MaxTokens:   32,
			}

			body, err := json.Marshal(payload)
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not encode request: %v\n", err)
				os.Exit(1)
			}

			req, err := http.NewRequest(
				"POST",
				"http://127.0.0.1:8080/v1/chat/completions",
				bytes.NewReader(body),
			)
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not create inference request: %v\n", err)
				os.Exit(1)
			}

			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Fprintf(os.Stderr, "inference request failed: %v\n", err)
				os.Exit(1)
			}

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				fmt.Fprintf(os.Stderr, "inference failed: HTTP %s\n", resp.Status)
				resp.Body.Close()
				os.Exit(1)
			}

			var result chatResponse

			err = json.NewDecoder(resp.Body).Decode(&result)
			resp.Body.Close()

			if err != nil {
				fmt.Fprintf(os.Stderr, "could not decode inference response: %v\n", err)
				os.Exit(1)
			}

			if len(result.Choices) == 0 {
				fmt.Fprintln(os.Stderr, "inference response contained no choices")
				os.Exit(1)
			}

			fmt.Println(result.Choices[0].Message.Content)

		default:
			fmt.Fprintf(os.Stderr, "unknown runtime command: %s\n", os.Args[2])
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
