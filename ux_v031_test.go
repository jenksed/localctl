package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLearnerNextStep(t *testing.T) {
	if got := learnerNextStep(nil, nil, "").Command; got != "localctl explore" {
		t.Fatalf("no-model next step: %s", got)
	}

	models := []modelArtifact{{ID: "granite", Name: "granite.gguf"}}
	if got := learnerNextStep(models, nil, "").Command; got != "localctl audition granite" {
		t.Fatalf("untested-model next step: %s", got)
	}

	records := []runObservation{{}}
	records[0].Model.ID = "granite"
	if got := learnerNextStep(models, records, "granite").Command; got != "localctl gaps granite" {
		t.Fatalf("tested-model next step: %s", got)
	}
}

func TestCandidateInstalledUsesModelIdentity(t *testing.T) {
	candidate := modelRadarCandidate{Name: "Qwen3.5-9B", Repository: "openresearchtools/Qwen3.5-9B-GGUF"}
	models := []modelArtifact{{ID: "qwen3-5-9b-q4-k-m", Name: "Qwen3.5-9B-Q4_K_M.gguf"}}
	if !candidateInstalled(candidate, models) {
		t.Fatal("expected installed Qwen candidate to be recognized")
	}
}

func TestExploreSelectionRoundTripAndInstalledPrune(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	candidate := m1ProRadar[0]
	state := exploreState{SchemaVersion: 1}
	state = selectExploreCandidate(state, candidate)
	if err := saveExploreState(state); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadExploreState()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Selected) != 1 || loaded.Selected[0].Candidate.Name != candidate.Name {
		t.Fatalf("unexpected loaded state: %#v", loaded)
	}
	pruned, changed := pruneInstalledSelections(loaded, []modelArtifact{{ID: modelSlug(candidate.Name) + "-q4-k-m"}})
	if !changed || len(pruned.Selected) != 0 {
		t.Fatalf("installed selection was not pruned: changed=%v state=%#v", changed, pruned)
	}
}

func TestCuratedExploreHidesInstalledAndSelected(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".lmstudio", "models", "test")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Qwen3.5-9B-Q4_K_M.gguf"), []byte("fake"), 0600); err != nil {
		t.Fatal(err)
	}
	state := exploreState{SchemaVersion: 1, Selected: []exploreSelection{{Candidate: m1ProRadar[1], SelectedAt: time.Now()}}}
	if err := saveExploreState(state); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runCuratedModelRadar(&stdout, &stderr); code != 0 {
		t.Fatalf("explore returned %d: %s", code, stderr.String())
	}
	output := stdout.String()
	if strings.Contains(output, "Qwen3.5-9B          priority") {
		t.Fatalf("installed candidate should not appear as new:\n%s", output)
	}
	if strings.Contains(output, "Gemma 4 E4B IT           priority") {
		t.Fatalf("selected candidate should not appear as new:\n%s", output)
	}
	if !strings.Contains(output, "Audit queue: Gemma 4 E4B IT") {
		t.Fatalf("selected candidate should be summarized in audit queue:\n%s", output)
	}
}
