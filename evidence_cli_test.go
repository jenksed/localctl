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

	item := exercise{
		ID:         "audit-test",
		Title:      "Audit test",
		Category:   "instruction",
		Difficulty: "easy",
		Prompt:     "Reply OK",
		Evaluation: evaluationSpec{Kind: evaluationExact, Expected: "OK"},
	}
	model := modelArtifact{
		ID:   "model",
		Name: "model.gguf",
		Path: filepath.Join(home, "model.gguf"),
		Size: 100,
	}
	outcome := inferenceOutcome{
		Content:      "OK",
		FinishReason: "stop",
		Elapsed:      10 * time.Millisecond,
	}
	if _, _, err := persistObservation(item, model, time.Now(), outcome, nil); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runEvidence([]string{"localctl", "evidence", "audit"}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected healthy audit, got %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "history status: COMPLETE") {
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
		t.Fatalf("expected healthy post-rebuild audit, got %d: %s", code, stderr.String())
	}
}
