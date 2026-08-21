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
	modelPath := filepath.Join(home, "test-model-Q4_K_M.gguf")
	if err := os.WriteFile(modelPath, []byte("test-model-artifact"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	model := modelArtifact{
		ID:   "test-model",
		Name: "test-model-Q4_K_M.gguf",
		Path: modelPath,
		Size: info.Size(),
	}
	outcome := inferenceOutcome{
		Model:               "test-model-Q4_K_M.gguf",
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
	if record.SchemaVersion != 3 {
		t.Fatalf("expected evidence schema 3, got %d", record.SchemaVersion)
	}
	if record.Experiment.ID == "" {
		t.Fatal("expected experiment identity")
	}
	if record.Exercise.Version != "v1" {
		t.Fatalf("expected exercise version v1, got %q", record.Exercise.Version)
	}
	if record.Pack.ID == "" || record.Pack.Version == "" {
		t.Fatalf("expected versioned pack identity, got %#v", record.Pack)
	}
	if record.Input.Class != "canonical" {
		t.Fatalf("expected canonical input class, got %q", record.Input.Class)
	}
	if record.Machine.OS == "" || record.Machine.Architecture == "" {
		t.Fatalf("expected machine fingerprint, got %#v", record.Machine)
	}
	if record.LocalCTL.Version != localctlVersion {
		t.Fatalf("expected LocalCTL version %s, got %q", localctlVersion, record.LocalCTL.Version)
	}
	if record.Model.SHA256 == "" {
		t.Fatal("expected model artifact content hash")
	}
	if record.Model.Quantization != "Q4_K_M" {
		t.Fatalf("expected quantization Q4_K_M, got %q", record.Model.Quantization)
	}
	if record.Profile.ID != "default" {
		t.Fatalf("expected default profile, got %q", record.Profile.ID)
	}
	if record.Validation.Authority != "deterministic" {
		t.Fatalf("expected deterministic validation authority, got %q", record.Validation.Authority)
	}
	if record.Evaluation.Status != "pass" {
		t.Fatalf("expected pass, got %#v", record.Evaluation)
	}
	if record.Exercise.PromptSHA256 == "" || record.Result.ResponseSHA256 == "" {
		t.Fatalf("expected prompt and response hashes, got %#v", record)
	}
	if record.Result.VisibleCharacters != len([]rune(outcome.Content)) {
		t.Fatalf("unexpected visible character count: %d", record.Result.VisibleCharacters)
	}
	if record.Result.EmptyVisibleOutput {
		t.Fatal("non-empty response was marked empty")
	}
	if record.Model.ArtifactMetadataKey == "" {
		t.Fatal("expected artifact metadata key")
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
