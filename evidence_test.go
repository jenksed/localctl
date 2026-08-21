package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistObservationAndJudgment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	item := exercise{
		ID:         "exact-output-v1",
		Title:      "Exact output discipline",
		Category:   "instruction",
		Difficulty: "easy",
		Prompt:     "Reply exactly LOCALCTL_OK",
		Evaluation: evaluationSpec{
			Kind:     evaluationExact,
			Expected: "LOCALCTL_OK",
		},
	}
	model := modelArtifact{
		ID:   "test-model",
		Name: "test-model.gguf",
		Path: filepath.Join(home, "test-model.gguf"),
		Size: 1234,
	}
	outcome := inferenceOutcome{
		Model:               "test-model.gguf",
		Content:             "LOCALCTL_OK",
		FinishReason:        "stop",
		PromptTokens:        4,
		CompletionTokens:    2,
		TotalTokens:         6,
		GenerationPerSecond: 20,
		Elapsed:             250 * time.Millisecond,
	}

	startedAt := time.Now()
	record, runDir, err := persistObservation(item, model, startedAt, outcome, nil)
	if err != nil {
		t.Fatal(err)
	}
	if record.Evaluation.Status != "pass" {
		t.Fatalf("expected pass, got %#v", record.Evaluation)
	}

	for _, name := range []string{"observation.json", "prompt.txt", "response.txt"} {
		if _, err := os.Stat(filepath.Join(runDir, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}

	records, err := listObservations()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].RunID != record.RunID {
		t.Fatalf("unexpected evidence index: %#v", records)
	}

	judgment, err := saveJudgment(record.RunID, "good", "deterministic result also looked useful")
	if err != nil {
		t.Fatal(err)
	}
	if judgment.Verdict != "good" {
		t.Fatalf("unexpected judgment: %#v", judgment)
	}

	loaded, err := loadJudgment(runDir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.Verdict != "good" {
		t.Fatalf("unexpected loaded judgment: %#v", loaded)
	}
}
