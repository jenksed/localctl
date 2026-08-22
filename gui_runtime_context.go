package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func performInferenceWithProfileContext(ctx context.Context, baseURL, requestedModel, prompt string, profile profileDefinition) (inferenceOutcome, error) {
	if profile.MaxTokens <= 0 {
		profile = defaultProfile()
	}
	payload := chatRequest{Model: requestedModel, Messages: []message{{Role: "user", Content: prompt}}, Temperature: profile.Temperature, MaxTokens: profile.MaxTokens}
	body, err := json.Marshal(payload)
	if err != nil {
		return inferenceOutcome{}, fmt.Errorf("could not encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return inferenceOutcome{}, fmt.Errorf("could not create inference request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	pid := 0
	if baseURL == runtimeURL {
		if state, stateErr := readRuntimeState(); stateErr == nil {
			pid = state.PID
		}
	}
	rssBefore := processResidentBytes(pid)
	startedAt := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return inferenceOutcome{Elapsed: time.Since(startedAt), RuntimeRSSBefore: rssBefore, RuntimeRSSAfter: processResidentBytes(pid)}, fmt.Errorf("inference request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return inferenceOutcome{Elapsed: time.Since(startedAt), RuntimeRSSBefore: rssBefore, RuntimeRSSAfter: processResidentBytes(pid)}, fmt.Errorf("inference failed: HTTP %s", resp.Status)
	}
	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return inferenceOutcome{Elapsed: time.Since(startedAt), RuntimeRSSBefore: rssBefore, RuntimeRSSAfter: processResidentBytes(pid)}, fmt.Errorf("could not decode inference response: %w", err)
	}
	if len(result.Choices) == 0 {
		return inferenceOutcome{Elapsed: time.Since(startedAt), RuntimeRSSBefore: rssBefore, RuntimeRSSAfter: processResidentBytes(pid)}, fmt.Errorf("inference response contained no choices")
	}
	choice := result.Choices[0]
	return inferenceOutcome{
		Model: result.Model, Content: choice.Message.Content, FinishReason: choice.FinishReason,
		PromptTokens: result.Usage.PromptTokens, CompletionTokens: result.Usage.CompletionTokens, TotalTokens: result.Usage.TotalTokens,
		PromptPerSecond: result.Timings.PromptPerSecond, GenerationPerSecond: result.Timings.PredictedPerSecond,
		Elapsed: time.Since(startedAt), RuntimeRSSBefore: rssBefore, RuntimeRSSAfter: processResidentBytes(pid),
	}, nil
}

func runObservedItemContext(ctx context.Context, item exercise, model modelArtifact, profile profileDefinition) (runObservation, inferenceOutcome, error) {
	return runObservedItemContextAt(ctx, runtimeURL, item, model, profile)
}

func runObservedItemContextAt(ctx context.Context, baseURL string, item exercise, model modelArtifact, profile profileDefinition) (runObservation, inferenceOutcome, error) {
	if scopedObservation != nil && scopedObservation.Pack.ID == "audition" {
		scopedObservation.Pack = inferredPackForExercise(item)
	}
	startedAt := time.Now()
	outcome, inferenceErr := performInferenceWithProfileContext(ctx, baseURL, model.Name, item.Prompt, profile)
	if ctx.Err() != nil {
		return runObservation{}, outcome, ctx.Err()
	}
	record, _, persistErr := persistObservation(item, model, startedAt, outcome, inferenceErr)
	if persistErr != nil {
		return runObservation{}, outcome, fmt.Errorf("save evidence: %w", persistErr)
	}
	if inferenceErr != nil {
		return record, outcome, inferenceErr
	}
	return record, outcome, nil
}
