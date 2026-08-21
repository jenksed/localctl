package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEvidenceAuditAndRebuildIndex(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	modelPath := filepath.Join(home, "model.gguf")
	if err := os.WriteFile(modelPath, []byte("test-model-artifact"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(modelPath)
	if err != nil {
		t.Fatal(err)
	}

	item := exercise{
		ID:         "audit-test-v1",
		Title:      "Audit test",
		Category:   "instruction",
		Difficulty: "easy",
		Prompt:     "Reply OK",
		Evaluation: evaluationSpec{Kind: evaluationExact, Expected: "OK"},
	}
	model := modelArtifact{ID: "model", Name: "model-Q4_K_M.gguf", Path: modelPath, Size: info.Size()}
	outcome := inferenceOutcome{Content: "OK", FinishReason: "stop", Elapsed: 10 * time.Millisecond}
	if _, _, err := persistObservation(item, model, time.Now(), outcome, nil); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runEvidence([]string{"localctl", "evidence", "audit"}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected healthy audit, got %d: %s\n%s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "history status: COMPLETE") || !strings.Contains(stdout.String(), "incomplete v3 records:  0") {
		t.Fatalf("unexpected audit output: %s", stdout.String())
	}

	root, err := evidenceRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "index.ndjson")); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runEvidence([]string{"localctl", "evidence", "audit"}, &stdout, &stderr); code != 1 {
		t.Fatalf("expected audit failure for missing index, got %d", code)
	}
	if !strings.Contains(stdout.String(), "NEEDS ATTENTION") {
		t.Fatalf("unexpected broken audit output: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runEvidence([]string{"localctl", "evidence", "rebuild-index"}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected rebuild success, got %d: %s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runEvidence([]string{"localctl", "evidence", "audit"}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected healthy post-rebuild audit, got %d: %s\n%s", code, stderr.String(), stdout.String())
	}
}

func TestV3AuditRejectsMissingProvenance(t *testing.T) {
	record := runObservation{SchemaVersion: 3}
	missing := v3ObservationMissing(record)
	if len(missing) == 0 {
		t.Fatal("expected empty v3 observation to report missing provenance")
	}
	joined := strings.Join(missing, ",")
	for _, expected := range []string{"experiment.id", "model.sha256", "profile.id", "validation.authority"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected %s in missing provenance: %s", expected, joined)
		}
	}
}
