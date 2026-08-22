package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func useTempLocalCTLHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func seedTestModel(t *testing.T, home, name string) modelArtifact {
	t.Helper()
	path := filepath.Join(home, ".lmstudio", "models", "test", name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	content := []byte("small deterministic GGUF fixture; not a real model")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	return modelArtifact{ID: modelSlug(name[:len(name)-len(filepath.Ext(name))]), Name: name, Path: path, Size: int64(len(content))}
}

func testModelSHA(model modelArtifact) string {
	data, _ := os.ReadFile(model.Path)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeTestObservation(t *testing.T, model modelArtifact, pack packManifest, item exercise, started time.Time, experimentID, status string) runObservation {
	t.Helper()
	record := runObservation{SchemaVersion: evidenceSchemaVersion, RunID: newRunID(started), StartedAt: started, CompletedAt: started.Add(time.Second)}
	record.Experiment.ID = experimentID
	record.Experiment.Name = pack.ID
	record.Experiment.Kind = "test"
	record.Exercise.ID = item.ID
	record.Exercise.Version = exerciseVersion(item)
	record.Exercise.Title = item.Title
	record.Exercise.Category = item.Category
	record.Exercise.Difficulty = item.Difficulty
	record.Exercise.PromptSHA256 = textSHA256(item.Prompt)
	record.Pack.ID = pack.ID
	record.Pack.Version = pack.Version
	record.Pack.Title = pack.Title
	record.Input.Class = "canonical"
	record.Machine = machineFingerprint()
	record.LocalCTL = localctlFingerprint()
	record.Model.ID = model.ID
	record.Model.Name = model.Name
	record.Model.Path = model.Path
	record.Model.Size = model.Size
	record.Model.SHA256 = testModelSHA(model)
	record.Model.Quantization = quantizationFromName(model.Name)
	record.Profile.ID = "default"
	record.Profile.Context = defaultProfile().Context
	record.Profile.MaxTokens = defaultProfile().MaxTokens
	record.Profile.Temperature = defaultProfile().Temperature
	record.Result.Status = "succeeded"
	record.Result.ElapsedMS = 100
	record.Result.GenerationTokensSec = 20
	record.Evaluation = evaluationResult{Mode: "deterministic", Status: status}
	record.Validation.Authority = "deterministic"
	record.Validation.Kind = "test"

	root, err := evidenceRoot()
	if err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, started.Format("2006"), started.Format("01"), started.Format("02"), record.RunID)
	if err := os.MkdirAll(runDir, 0700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.MarshalIndent(record, "", "  ")
	if err := os.WriteFile(filepath.Join(runDir, "observation.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := appendObservationIndex(root, record); err != nil {
		t.Fatal(err)
	}
	return record
}

func seedSupportedPack(t *testing.T, model modelArtifact, packID string, started time.Time) []runObservation {
	t.Helper()
	pack, err := findPack(packID)
	if err != nil {
		t.Fatal(err)
	}
	items := packExercises(pack)
	var deterministic []exercise
	for _, item := range items {
		if item.Evaluation.Kind != evaluationManual {
			deterministic = append(deterministic, item)
		}
	}
	if len(deterministic) == 0 {
		t.Fatalf("pack %s has no deterministic exercises", packID)
	}
	var records []runObservation
	index := 0
	for len(records) < 8 || index < len(deterministic) {
		item := deterministic[index%len(deterministic)]
		records = append(records, writeTestObservation(t, model, pack, item, started.Add(time.Duration(index)*time.Millisecond), "exp_supported", "pass"))
		index++
	}
	return records
}

func findTerritoryCell(t *testing.T, territory territoryProjection, id string) territoryCell {
	t.Helper()
	for _, cell := range territory.Cells {
		if cell.ID == id {
			return cell
		}
	}
	t.Fatalf("territory cell %s not found", id)
	return territoryCell{}
}

func TestTerritoryUsesRealCapabilityStateAndSourceProvenance(t *testing.T) {
	home := useTempLocalCTLHome(t)
	model := seedTestModel(t, home, "test-coder-Q4_K_M.gguf")
	records := seedSupportedPack(t, model, "developer-core", time.Now().Add(-time.Hour))
	app := newLocalApplication()
	territory, err := app.GetTerritory(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	coding := findTerritoryCell(t, territory, "coding")
	if coding.State != "SUPPORTED" && coding.State != "STRONG" {
		t.Fatalf("coding state = %s, want supported/strong", coding.State)
	}
	if coding.EvidenceLeader == nil || coding.EvidenceLeader.ModelID != model.ID {
		t.Fatalf("unexpected evidence leader: %#v", coding.EvidenceLeader)
	}
	if len(coding.SourceRunIDs) != len(records) {
		t.Fatalf("source run ids = %d, want %d", len(coding.SourceRunIDs), len(records))
	}
	if coding.RuleVersion != capabilityRuleVersion {
		t.Fatalf("rule version = %s", coding.RuleVersion)
	}
	agent := findTerritoryCell(t, territory, "agent-tool-use")
	if agent.State != "UNKNOWN" || agent.EvidenceLeader != nil {
		t.Fatalf("tool use must remain unknown without a canonical pack: %#v", agent)
	}
}

func TestTerritorySurfacesStaleEvidenceAsStale(t *testing.T) {
	home := useTempLocalCTLHome(t)
	model := seedTestModel(t, home, "stale-coder-Q4_K_M.gguf")
	seedSupportedPack(t, model, "developer-core", time.Now().Add(-60*24*time.Hour))
	territory, err := newLocalApplication().GetTerritory(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	if got := findTerritoryCell(t, territory, "coding").State; got != "STALE" {
		t.Fatalf("coding state = %s, want STALE", got)
	}
}

func TestReadIndexCanBeDeletedAndRebuiltWithoutDeletingEvidence(t *testing.T) {
	home := useTempLocalCTLHome(t)
	model := seedTestModel(t, home, "index-Q4_K_M.gguf")
	pack, _ := findPack("structured-output")
	item := packExercises(pack)[0]
	record := writeTestObservation(t, model, pack, item, time.Now(), "exp_index", "pass")
	root, _ := evidenceRoot()
	indexPath := filepath.Join(root, "index.ndjson")
	if err := os.Remove(indexPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, record.StartedAt.Format("2006"), record.StartedAt.Format("01"), record.StartedAt.Format("02"), record.RunID, "observation.json")); err != nil {
		t.Fatalf("source evidence disappeared with index: %v", err)
	}
	count, err := newLocalApplication().RebuildReadIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("rebuilt %d records, want 1", count)
	}
	runs, err := listObservations()
	if err != nil || len(runs) != 1 || runs[0].RunID != record.RunID {
		t.Fatalf("rebuilt query result = %#v, err=%v", runs, err)
	}
}

func TestMissionPlanSkipsSatisfiedEvidenceAndDoesNotUseProxyForToolUse(t *testing.T) {
	home := useTempLocalCTLHome(t)
	model := seedTestModel(t, home, "mission-Q4_K_M.gguf")
	now := time.Now().Add(-time.Hour)
	seedSupportedPack(t, model, "developer-core", now)
	app := newLocalApplication()
	plan, err := app.PlanMission(context.Background(), "developer", model.ID, "default")
	if err != nil {
		t.Fatal(err)
	}
	for _, packID := range plan.RunnablePacks {
		if packID == "developer-core" {
			t.Fatal("satisfied developer-core pack was unnecessarily scheduled")
		}
	}

	seedSupportedPack(t, model, "structured-output", now.Add(time.Minute))
	seedSupportedPack(t, model, "reasoning-analysis", now.Add(2*time.Minute))
	agentPlan, err := app.PlanMission(context.Background(), "coding-agent", model.ID, "default")
	if err != nil {
		t.Fatal(err)
	}
	if agentPlan.Complete {
		t.Fatal("coding-agent mission completed from proxy evidence despite missing tool-use evidence")
	}
	if len(agentPlan.Unresolved) != 1 || agentPlan.Unresolved[0] != "agent-tool-use" {
		t.Fatalf("unresolved = %#v", agentPlan.Unresolved)
	}
}
