package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type capabilitySnapshotSummary struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	GeneratedAt time.Time `json:"generated_at"`
	ModelID     string    `json:"model_id"`
	ProfileID   string    `json:"profile_id"`
	RuleVersion string    `json:"rule_version"`
}

func intelligenceRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".localctl", "intelligence"), nil
}

func capabilitySnapshotID(capabilities modelCapabilityMap) string {
	stamp := capabilities.GeneratedAt.UTC().Format("20060102T150405.000000000")
	model := modelSlug(capabilities.ModelID)
	if model == "" {
		model = "unknown-model"
	}
	profile := modelSlug(capabilities.ProfileID)
	if profile == "" {
		profile = "default"
	}
	return fmt.Sprintf("cap_%s_%s_%s", stamp, model, profile)
}

func persistCapabilityMap(capabilities modelCapabilityMap) (string, string, error) {
	root, err := intelligenceRoot()
	if err != nil {
		return "", "", err
	}
	id := capabilitySnapshotID(capabilities)
	dir := filepath.Join(root, "capability", capabilities.GeneratedAt.Format("2006"), capabilities.GeneratedAt.Format("01"), capabilities.GeneratedAt.Format("02"))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", err
	}
	path := filepath.Join(dir, id+".json")
	data, err := json.MarshalIndent(capabilities, "", "  ")
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", "", err
	}
	return id, path, nil
}

func listCapabilitySnapshots() ([]capabilitySnapshotSummary, error) {
	root, err := intelligenceRoot()
	if err != nil {
		return nil, err
	}
	base := filepath.Join(root, "capability")
	var snapshots []capabilitySnapshotSummary
	err = filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		var capability modelCapabilityMap
		if decodeErr := json.Unmarshal(data, &capability); decodeErr != nil {
			return fmt.Errorf("decode capability snapshot %s: %w", path, decodeErr)
		}
		snapshots = append(snapshots, capabilitySnapshotSummary{
			ID:          strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			Path:        path,
			GeneratedAt: capability.GeneratedAt,
			ModelID:     capability.ModelID,
			ProfileID:   capability.ProfileID,
			RuleVersion: capability.RuleVersion,
		})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].GeneratedAt.After(snapshots[j].GeneratedAt) })
	return snapshots, nil
}

func findCapabilitySnapshot(reference string) (capabilitySnapshotSummary, modelCapabilityMap, error) {
	snapshots, err := listCapabilitySnapshots()
	if err != nil {
		return capabilitySnapshotSummary{}, modelCapabilityMap{}, err
	}
	var matches []capabilitySnapshotSummary
	for _, snapshot := range snapshots {
		if snapshot.ID == reference || strings.HasPrefix(snapshot.ID, reference) {
			matches = append(matches, snapshot)
		}
	}
	if len(matches) == 0 {
		return capabilitySnapshotSummary{}, modelCapabilityMap{}, fmt.Errorf("capability snapshot %q not found", reference)
	}
	if len(matches) > 1 {
		return capabilitySnapshotSummary{}, modelCapabilityMap{}, fmt.Errorf("capability snapshot reference %q is ambiguous", reference)
	}
	data, err := os.ReadFile(matches[0].Path)
	if err != nil {
		return capabilitySnapshotSummary{}, modelCapabilityMap{}, err
	}
	var capability modelCapabilityMap
	if err := json.Unmarshal(data, &capability); err != nil {
		return capabilitySnapshotSummary{}, modelCapabilityMap{}, err
	}
	return matches[0], capability, nil
}

func auditCapabilitySnapshots() (int, int, []string, error) {
	snapshots, err := listCapabilitySnapshots()
	if err != nil {
		return 0, 0, nil, err
	}
	problems := []string{}
	sourceRuns := 0
	for _, snapshot := range snapshots {
		_, capability, loadErr := findCapabilitySnapshot(snapshot.ID)
		if loadErr != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", snapshot.ID, loadErr))
			continue
		}
		if capability.SchemaVersion != 1 {
			problems = append(problems, fmt.Sprintf("%s: unsupported capability snapshot schema %d", snapshot.ID, capability.SchemaVersion))
		}
		if capability.RuleVersion == "" {
			problems = append(problems, fmt.Sprintf("%s: missing rule_version", snapshot.ID))
		}
		for _, assessment := range capability.Assessments {
			if assessment.ComparableRuns != len(assessment.SourceRunIDs) {
				problems = append(problems, fmt.Sprintf("%s/%s: comparable_runs=%d but source_run_ids=%d", snapshot.ID, assessment.PackID, assessment.ComparableRuns, len(assessment.SourceRunIDs)))
			}
			for _, runID := range assessment.SourceRunIDs {
				sourceRuns++
				if _, _, findErr := findObservation(runID); findErr != nil {
					problems = append(problems, fmt.Sprintf("%s/%s: source run %s not found", snapshot.ID, assessment.PackID, runID))
				}
			}
		}
	}
	return len(snapshots), sourceRuns, problems, nil
}

func runIntelligence(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl intelligence <list|show|audit> ...")
		return 1
	}
	switch args[2] {
	case "list":
		snapshots, err := listCapabilitySnapshots()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "SNAPSHOT                                      GENERATED             MODEL                    PROFILE       RULE")
		limit := len(snapshots)
		if limit > 20 {
			limit = 20
		}
		for _, snapshot := range snapshots[:limit] {
			fmt.Fprintf(stdout, "%-45s %-21s %-24s %-13s %s\n", shorten(snapshot.ID, 45), snapshot.GeneratedAt.Format(time.RFC3339), shorten(snapshot.ModelID, 24), snapshot.ProfileID, snapshot.RuleVersion)
		}
		return 0
	case "show":
		if len(args) < 4 {
			fmt.Fprintln(stderr, "usage: localctl intelligence show <snapshot-id>")
			return 1
		}
		snapshot, capability, err := findCapabilitySnapshot(args[3])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		data, _ := json.MarshalIndent(capability, "", "  ")
		fmt.Fprintf(stdout, "snapshot: %s\npath: %s\n\n", snapshot.ID, snapshot.Path)
		fmt.Fprintln(stdout, string(data))
		return 0
	case "audit":
		snapshots, sourceRuns, problems, err := auditCapabilitySnapshots()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "capability snapshots: %d\nsource run references: %d\n", snapshots, sourceRuns)
		if len(problems) == 0 {
			fmt.Fprintln(stdout, "intelligence status: COMPLETE")
			return 0
		}
		fmt.Fprintf(stdout, "intelligence status: NEEDS ATTENTION (%d problems)\n", len(problems))
		for _, problem := range problems {
			fmt.Fprintf(stdout, "- %s\n", problem)
		}
		return 1
	default:
		fmt.Fprintf(stderr, "unknown intelligence command: %s\n", args[2])
		return 1
	}
}
