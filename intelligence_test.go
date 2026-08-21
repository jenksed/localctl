package main

import (
	"os"
	"testing"
	"time"
)

func TestCapabilityAssessmentRecordsExactSourceRuns(t *testing.T) {
	pack, err := findPack("structured-output")
	if err != nil {
		t.Fatal(err)
	}
	profile := defaultProfile()
	machine := machineFingerprintRecord{OS: "darwin", Architecture: "arm64", Chip: "Apple M1 Pro", MemoryBytes: 16}
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	good := capabilityTestRecord(pack, profile, machine, "sha-a", "json-object-v1", "exp-a", now, "pass")
	good.RunID = "run-good"
	excluded := good
	excluded.RunID = "run-wrong-profile"
	excluded.Profile.ID = "fast"

	assessment := deriveCapabilityAssessment(pack, profile, machine, "sha-a", now, []runObservation{excluded, good}, nil)
	if assessment.ComparableRuns != 1 || len(assessment.SourceRunIDs) != 1 {
		t.Fatalf("expected one comparable source, got %#v", assessment)
	}
	if assessment.SourceRunIDs[0] != "run-good" {
		t.Fatalf("unexpected source provenance: %#v", assessment.SourceRunIDs)
	}
}

func TestCandidateRolesRequireCurrentSupportedEvidence(t *testing.T) {
	assessments := []capabilityAssessment{
		{PackID: "developer-core", State: "SUPPORTED", Freshness: "CURRENT"},
		{PackID: "linux-investigation", State: "STRONG", Freshness: "STALE"},
		{PackID: "docker-investigation", State: "PROMISING", Freshness: "CURRENT"},
	}
	roles := candidateRoles(assessments)
	if len(roles) != 1 || roles[0] != "developer helper" {
		t.Fatalf("expected only current supported developer role, got %#v", roles)
	}
}

func TestCapabilitySnapshotRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	generated := time.Date(2026, 8, 21, 15, 4, 5, 123456789, time.UTC)
	capability := modelCapabilityMap{
		SchemaVersion: 1,
		RuleVersion:   capabilityRuleVersion,
		GeneratedAt:   generated,
		ModelID:       "granite-test",
		ModelName:     "granite-test.gguf",
		ProfileID:     "default",
		Machine:       machineFingerprintRecord{OS: "darwin", Architecture: "arm64"},
		Assessments: []capabilityAssessment{
			{PackID: "developer-core", PackVersion: "v1", State: "SUPPORTED", ComparableRuns: 0},
		},
	}
	id, path, err := persistCapabilityMap(capability)
	if err != nil {
		t.Fatal(err)
	}
	if id == "" || path == "" {
		t.Fatalf("expected snapshot identity and path, got id=%q path=%q", id, path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected persisted snapshot: %v", err)
	}

	summary, loaded, err := findCapabilitySnapshot(id)
	if err != nil {
		t.Fatal(err)
	}
	if summary.RuleVersion != capabilityRuleVersion || loaded.RuleVersion != capabilityRuleVersion {
		t.Fatalf("lost rule provenance: summary=%#v loaded=%#v", summary, loaded)
	}
	if loaded.ModelID != capability.ModelID || loaded.GeneratedAt != generated {
		t.Fatalf("unexpected round trip: %#v", loaded)
	}
}

func TestIntelligenceAuditDetectsMissingSourceRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	capability := modelCapabilityMap{
		SchemaVersion: 1,
		RuleVersion:   capabilityRuleVersion,
		GeneratedAt:   time.Now(),
		ModelID:       "test-model",
		ProfileID:     "default",
		Machine:       machineFingerprintRecord{OS: "darwin", Architecture: "arm64"},
		Assessments: []capabilityAssessment{
			{PackID: "developer-core", PackVersion: "v1", ComparableRuns: 1, SourceRunIDs: []string{"run_missing"}},
		},
	}
	if _, _, err := persistCapabilityMap(capability); err != nil {
		t.Fatal(err)
	}
	snapshots, sourceRuns, problems, err := auditCapabilitySnapshots()
	if err != nil {
		t.Fatal(err)
	}
	if snapshots != 1 || sourceRuns != 1 {
		t.Fatalf("unexpected audit counts: snapshots=%d sourceRuns=%d", snapshots, sourceRuns)
	}
	if len(problems) == 0 {
		t.Fatal("expected audit to detect missing source observation")
	}
}
