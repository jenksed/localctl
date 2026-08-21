package main

import (
	"testing"
	"time"
)

func capabilityTestRecord(pack packManifest, profile profileDefinition, machine machineFingerprintRecord, sha, exerciseID, experimentID string, started time.Time, status string) runObservation {
	record := runObservation{SchemaVersion: 3, RunID: "run-" + exerciseID + "-" + experimentID, StartedAt: started, CompletedAt: started.Add(time.Second)}
	record.Input.Class = "canonical"
	record.Pack.ID = pack.ID
	record.Pack.Version = pack.Version
	record.Pack.Title = pack.Title
	record.Profile.ID = profile.ID
	record.Profile.Context = profile.Context
	record.Profile.Temperature = profile.Temperature
	record.Profile.MaxTokens = profile.MaxTokens
	record.Machine = machine
	record.Model.SHA256 = sha
	record.Experiment.ID = experimentID
	record.Exercise.ID = exerciseID
	record.Exercise.Version = "v1"
	record.Result.Status = "succeeded"
	record.Result.ElapsedMS = 100
	record.Result.GenerationTokensSec = 20
	record.Evaluation = evaluationResult{Mode: "exact", Status: status}
	return record
}

func TestCapabilityStrongRequiresExactCurrentBoundary(t *testing.T) {
	pack, err := findPack("structured-output")
	if err != nil {
		t.Fatal(err)
	}
	profile := defaultProfile()
	machine := machineFingerprintRecord{OS: "darwin", Architecture: "arm64", Chip: "Apple M1 Pro", MemoryBytes: 16 * 1024 * 1024 * 1024}
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	var records []runObservation
	for _, item := range packExercises(pack) {
		if item.Evaluation.Kind == evaluationManual {
			continue
		}
		records = append(records, capabilityTestRecord(pack, profile, machine, "sha-a", item.ID, "exp-a", now.Add(-24*time.Hour), "pass"))
	}
	for _, item := range packExercises(pack) {
		if item.Evaluation.Kind == evaluationManual {
			continue
		}
		records = append(records, capabilityTestRecord(pack, profile, machine, "sha-a", item.ID, "exp-b", now.Add(-12*time.Hour), "pass"))
		break
	}

	assessment := deriveCapabilityAssessment(pack, profile, machine, "sha-a", now, records, nil)
	if assessment.State != "STRONG" {
		t.Fatalf("expected STRONG, got %#v", assessment)
	}
	if assessment.Strength != "STRONG" {
		t.Fatalf("expected strong evidence strength, got %s", assessment.Strength)
	}
	if assessment.Coverage < 0.99 {
		t.Fatalf("expected full deterministic coverage, got %.3f", assessment.Coverage)
	}
	if assessment.Experiments != 2 {
		t.Fatalf("expected two experiments, got %d", assessment.Experiments)
	}
}

func TestCapabilityExcludesMismatchedProfileMachineArtifactAndPrivate(t *testing.T) {
	pack, err := findPack("developer-core")
	if err != nil {
		t.Fatal(err)
	}
	profile := defaultProfile()
	machine := machineFingerprintRecord{OS: "darwin", Architecture: "arm64", Chip: "Apple M1 Pro", MemoryBytes: 16}
	now := time.Now()
	base := capabilityTestRecord(pack, profile, machine, "sha-a", "go-defer-order-v1", "exp-a", now, "pass")

	wrongProfile := base
	wrongProfile.RunID = "wrong-profile"
	wrongProfile.Profile.ID = "fast"
	wrongMachine := base
	wrongMachine.RunID = "wrong-machine"
	wrongMachine.Machine.Chip = "Apple M3"
	wrongArtifact := base
	wrongArtifact.RunID = "wrong-artifact"
	wrongArtifact.Model.SHA256 = "sha-b"
	private := base
	private.RunID = "private"
	private.Input.Class = "private"

	assessment := deriveCapabilityAssessment(pack, profile, machine, "sha-a", now, []runObservation{base, wrongProfile, wrongMachine, wrongArtifact, private}, nil)
	if assessment.ComparableRuns != 1 {
		t.Fatalf("expected exactly one comparable run, got %d", assessment.ComparableRuns)
	}
	if assessment.ExcludedRuns != 4 {
		t.Fatalf("expected four excluded runs, got %d", assessment.ExcludedRuns)
	}
}

func TestCapabilityFreshnessCanInvalidateOtherwiseGoodEvidence(t *testing.T) {
	pack, err := findPack("reasoning-analysis")
	if err != nil {
		t.Fatal(err)
	}
	profile := defaultProfile()
	machine := machineFingerprintRecord{OS: "darwin", Architecture: "arm64"}
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	var records []runObservation
	for _, item := range packExercises(pack) {
		if item.Evaluation.Kind == evaluationManual {
			continue
		}
		records = append(records, capabilityTestRecord(pack, profile, machine, "sha-a", item.ID, "exp-old", now.Add(-60*24*time.Hour), "pass"))
	}
	assessment := deriveCapabilityAssessment(pack, profile, machine, "sha-a", now, records, nil)
	if assessment.Freshness != "STALE" || assessment.State != "STALE" {
		t.Fatalf("expected stale capability, got freshness=%s state=%s", assessment.Freshness, assessment.State)
	}
}

func TestCapabilityMixedAndWeakStates(t *testing.T) {
	base := capabilityAssessment{ComparableRuns: 10, SuccessfulInference: 10, Scored: 10, Coverage: 0.8, Strength: "SUPPORTED", Freshness: "CURRENT"}
	base.InferenceSuccessRate = 1

	mixed := base
	mixed.Pass = 7
	mixed.Fail = 3
	mixed.PassRate = 0.7
	if got := capabilityState(mixed); got != "MIXED" {
		t.Fatalf("expected MIXED, got %s", got)
	}

	weak := base
	weak.Pass = 5
	weak.Fail = 5
	weak.PassRate = 0.5
	if got := capabilityState(weak); got != "WEAK" {
		t.Fatalf("expected WEAK, got %s", got)
	}
}

func TestRequalificationPriorityPrefersStaleThenUnknown(t *testing.T) {
	if stale := requalificationPriority("STRONG", "STALE"); stale != 100 {
		t.Fatalf("expected stale priority 100, got %d", stale)
	}
	if unknown := requalificationPriority("UNKNOWN", "UNKNOWN"); unknown != 90 {
		t.Fatalf("expected unknown priority 90, got %d", unknown)
	}
	if current := requalificationPriority("SUPPORTED", "CURRENT"); current != 0 {
		t.Fatalf("expected current supported capability not to require refresh, got %d", current)
	}
}

func TestRecommendationRankingPrefersSupportedEvidence(t *testing.T) {
	if recommendationStateRank("SUPPORTED") <= recommendationStateRank("PROMISING") {
		t.Fatal("SUPPORTED must outrank PROMISING")
	}
	if recommendationStateRank("STALE") >= recommendationStateRank("WEAK") {
		t.Fatal("STALE evidence must not outrank current WEAK evidence")
	}
}
