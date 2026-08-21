package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunInsightsAggregatesFailureKinds(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	modelPath := filepath.Join(home, "test-model.gguf")
	if err := os.WriteFile(modelPath, []byte("test-model-artifact"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	model := modelArtifact{
		ID:   "test-model",
		Name: "test-model.gguf",
		Path: modelPath,
		Size: info.Size(),
	}

	passItem := exercise{
		ID:         "pass-v1",
		Title:      "Pass",
		Category:   "instruction",
		Difficulty: "easy",
		Prompt:     "Reply YES",
		Evaluation: evaluationSpec{Kind: evaluationExact, Expected: "YES"},
	}
	passOutcome := inferenceOutcome{
		Content:             "YES",
		FinishReason:        "stop",
		Elapsed:             100 * time.Millisecond,
		GenerationPerSecond: 20,
	}
	passRecord, _, err := persistObservation(passItem, model, time.Now(), passOutcome, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := saveJudgment(passRecord.RunID, "good", "useful"); err != nil {
		t.Fatal(err)
	}

	failItem := exercise{
		ID:         "extra-output-v1",
		Title:      "Extra output",
		Category:   "instruction",
		Difficulty: "easy",
		Prompt:     "Reply NO",
		Evaluation: evaluationSpec{Kind: evaluationExact, Expected: "NO"},
	}
	failOutcome := inferenceOutcome{
		Content:             "NO\n\nExplanation: extra text",
		FinishReason:        "stop",
		Elapsed:             200 * time.Millisecond,
		GenerationPerSecond: 10,
	}
	if _, _, err := persistObservation(failItem, model, time.Now().Add(time.Millisecond), failOutcome, nil); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runInsights([]string{"localctl", "insights"}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	output := stdout.String()
	for _, expected := range []string{
		"runs:                 2",
		"auto pass:             1",
		"auto fail:             1",
		"instruction",
		"contract_extra_output",
		"good:    1",
		"v3: 2 runs",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in insights output:\n%s", expected, output)
		}
	}
}
