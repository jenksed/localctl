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

type experimentRecord struct {
	SchemaVersion int       `json:"schema_version"`
	ID            string    `json:"id"`
	SessionID     string    `json:"session_id,omitempty"`
	Name          string    `json:"name"`
	Kind          string    `json:"kind"`
	ModelID       string    `json:"model_id,omitempty"`
	ModelPath     string    `json:"model_path,omitempty"`
	ProfileID     string    `json:"profile_id"`
	PackID        string    `json:"pack_id,omitempty"`
	PackVersion   string    `json:"pack_version,omitempty"`
	InputClass    string    `json:"input_class"`
	StartedAt     time.Time `json:"started_at"`
	CompletedAt   time.Time `json:"completed_at,omitempty"`
	Status        string    `json:"status"`
}

type observationScope struct {
	ExperimentID string
	SessionID    string
	Experiment   string
	ExperimentKind string
	Profile      profileDefinition
	Pack         packManifest
	InputClass   string
}

var scopedObservation *observationScope

func experimentRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".localctl", "experiments"), nil
}

func activeExperimentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".localctl", "active-experiment.json"), nil
}

func newExperimentID(now time.Time) string {
	return strings.Replace(newRunID(now), "run_", "exp_", 1)
}

func saveExperiment(record experimentRecord) error {
	root, err := experimentRoot()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, record.ID+".json"), data, 0600)
}

func startScopedExperiment(kind, name string, model modelArtifact, profile profileDefinition, pack packManifest, inputClass string) (experimentRecord, func()) {
	now := time.Now()
	record := experimentRecord{
		SchemaVersion: 1,
		ID:            newExperimentID(now),
		Name:          name,
		Kind:          kind,
		ModelID:       model.ID,
		ModelPath:     model.Path,
		ProfileID:     profile.ID,
		PackID:        pack.ID,
		PackVersion:   pack.Version,
		InputClass:    inputClass,
		StartedAt:     now,
		Status:        "running",
	}
	_ = saveExperiment(record)
	previous := scopedObservation
	scopedObservation = &observationScope{
		ExperimentID: record.ID,
		SessionID: record.SessionID,
		Experiment: record.Name,
		ExperimentKind: record.Kind,
		Profile: profile,
		Pack: pack,
		InputClass: inputClass,
	}
	finish := func() {
		record.CompletedAt = time.Now()
		record.Status = "completed"
		_ = saveExperiment(record)
		scopedObservation = previous
	}
	return record, finish
}

func loadActiveExperiment() (*experimentRecord, error) {
	path, err := activeExperimentPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var record experimentRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func currentObservationScope(item exercise) observationScope {
	if scopedObservation != nil {
		return *scopedObservation
	}
	if record, err := loadActiveExperiment(); err == nil && record != nil && record.Status == "running" {
		profile, profileErr := resolveProfile(record.ProfileID)
		if profileErr != nil {
			profile = defaultProfile()
		}
		pack, _ := findPack(record.PackID)
		return observationScope{
			ExperimentID: record.ID,
			SessionID: record.SessionID,
			Experiment: record.Name,
			ExperimentKind: record.Kind,
			Profile: profile,
			Pack: pack,
			InputClass: record.InputClass,
		}
	}
	profile := defaultProfile()
	pack := inferredPackForExercise(item)
	return observationScope{
		ExperimentID: newExperimentID(time.Now()),
		Experiment: "single run",
		ExperimentKind: "single",
		Profile: profile,
		Pack: pack,
		InputClass: inferredInputClass(item),
	}
}

func inferredInputClass(item exercise) string {
	if item.Category == "freeform" || strings.HasPrefix(item.Category, "work") || item.ID == "runtime-infer" {
		return "private"
	}
	return "canonical"
}

func runExperiment(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl experiment <start|status|finish|list> ...")
		return 1
	}
	switch args[2] {
	case "start":
		if len(args) < 4 {
			fmt.Fprintln(stderr, "usage: localctl experiment start <name> [model] [--profile=default] [--pack=...]")
			return 1
		}
		if current, err := loadActiveExperiment(); err == nil && current != nil {
			fmt.Fprintf(stderr, "experiment %s is already active; finish it first\n", current.ID)
			return 1
		}
		name := args[3]
		modelRef := ""
		profileRef := "default"
		packRef := ""
		for _, arg := range args[4:] {
			switch {
			case strings.HasPrefix(arg, "--profile="):
				profileRef = strings.TrimPrefix(arg, "--profile=")
			case strings.HasPrefix(arg, "--pack="):
				packRef = strings.TrimPrefix(arg, "--pack=")
			case modelRef == "":
				modelRef = arg
			default:
				fmt.Fprintf(stderr, "unexpected experiment argument: %s\n", arg)
				return 1
			}
		}
		profile, err := resolveProfile(profileRef)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		pack, err := findPack(packRef)
		if err != nil && packRef != "" {
			fmt.Fprintln(stderr, err)
			return 1
		}
		record := experimentRecord{SchemaVersion: 1, ID: newExperimentID(time.Now()), Name: name, Kind: "manual", ProfileID: profile.ID, PackID: pack.ID, PackVersion: pack.Version, InputClass: "canonical", StartedAt: time.Now(), Status: "running"}
		if modelRef != "" {
			model, modelErr := resolveModel(modelRef)
			if modelErr != nil {
				fmt.Fprintln(stderr, modelErr)
				return 1
			}
			record.ModelID = model.ID
			record.ModelPath = model.Path
		}
		if err := saveExperiment(record); err != nil {
			fmt.Fprintf(stderr, "could not save experiment: %v\n", err)
			return 1
		}
		path, _ := activeExperimentPath()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		data, _ := json.MarshalIndent(record, "", "  ")
		if err := os.WriteFile(path, data, 0600); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "experiment started: %s\n", record.ID)
		fmt.Fprintf(stdout, "name: %s\nprofile: %s\n", record.Name, record.ProfileID)
		return 0
	case "status":
		record, err := loadActiveExperiment()
		if err != nil {
			fmt.Fprintf(stderr, "could not read active experiment: %v\n", err)
			return 1
		}
		if record == nil {
			fmt.Fprintln(stdout, "no active manual experiment")
			return 0
		}
		data, _ := json.MarshalIndent(record, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	case "finish":
		record, err := loadActiveExperiment()
		if err != nil || record == nil {
			fmt.Fprintln(stderr, "no active manual experiment")
			return 1
		}
		record.Status = "completed"
		record.CompletedAt = time.Now()
		if err := saveExperiment(*record); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		path, _ := activeExperimentPath()
		_ = os.Remove(path)
		fmt.Fprintf(stdout, "experiment finished: %s\n", record.ID)
		return 0
	case "list":
		records, err := listExperiments()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "EXPERIMENT                              STATUS      MODEL                    PROFILE       NAME")
		limit := len(records)
		if limit > 20 {
			limit = 20
		}
		for _, record := range records[:limit] {
			fmt.Fprintf(stdout, "%-39s %-11s %-24s %-13s %s\n", record.ID, record.Status, shorten(record.ModelID, 24), record.ProfileID, record.Name)
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown experiment command: %s\n", args[2])
		return 1
	}
}

func listExperiments() ([]experimentRecord, error) {
	root, err := experimentRoot()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var records []experimentRecord
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(root, entry.Name()))
		if readErr != nil {
			return nil, readErr
		}
		var record experimentRecord
		if decodeErr := json.Unmarshal(data, &record); decodeErr != nil {
			return nil, decodeErr
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].StartedAt.After(records[j].StartedAt) })
	return records, nil
}
