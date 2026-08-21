package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

type categoryCharacterization struct {
	Category string `json:"category"`
	Runs     int    `json:"runs"`
	Scored   int    `json:"scored"`
	Pass     int    `json:"pass"`
	Fail     int    `json:"fail"`
	Pending  int    `json:"pending"`
	Coverage string `json:"coverage"`
}

type modelCharacterization struct {
	SchemaVersion      int                        `json:"schema_version"`
	GeneratedAt        time.Time                  `json:"generated_at"`
	ModelID            string                     `json:"model_id"`
	ModelName          string                     `json:"model_name"`
	Runs               int                        `json:"runs"`
	CanonicalRuns      int                        `json:"canonical_runs"`
	PrivateRuns        int                        `json:"private_runs"`
	SchemaVersions     map[int]int                `json:"schema_versions"`
	Profiles           map[string]int             `json:"profiles"`
	Experiments        int                        `json:"experiments"`
	Categories         []categoryCharacterization `json:"categories"`
	FailureKinds       map[string]int             `json:"failure_kinds"`
	MedianElapsed      string                     `json:"median_elapsed"`
	MedianGeneration   string                     `json:"median_generation"`
	RuntimeRSSSamples  int                        `json:"runtime_rss_samples"`
	MaxRuntimeRSSBytes int64                      `json:"max_runtime_rss_bytes,omitempty"`
	MachineMemoryBytes int64                      `json:"machine_memory_bytes,omitempty"`
	LatestRun          time.Time                  `json:"latest_run,omitempty"`
}

func historicalModelRecords(reference string) (string, string, []runObservation, error) {
	records, err := listObservations()
	if err != nil {
		return "", "", nil, err
	}
	if model, resolveErr := resolveModel(reference); resolveErr == nil {
		var filtered []runObservation
		for _, record := range records {
			if record.Model.Path == model.Path || record.Model.ID == model.ID || record.Model.Name == model.Name {
				filtered = append(filtered, record)
			}
		}
		return model.ID, model.Name, filtered, nil
	}
	needle := strings.ToLower(reference)
	type identity struct{ id, name string }
	identities := map[string]identity{}
	for _, record := range records {
		if strings.Contains(strings.ToLower(record.Model.ID), needle) || strings.Contains(strings.ToLower(record.Model.Name), needle) || strings.Contains(strings.ToLower(record.Model.Path), needle) {
			key := record.Model.ID + "\x00" + record.Model.Name
			identities[key] = identity{id: record.Model.ID, name: record.Model.Name}
		}
	}
	if len(identities) == 0 {
		return "", "", nil, fmt.Errorf("no installed or historical model matches %q", reference)
	}
	if len(identities) > 1 {
		var names []string
		for _, item := range identities {
			names = append(names, item.name)
		}
		sort.Strings(names)
		return "", "", nil, fmt.Errorf("historical model reference %q is ambiguous: %s", reference, strings.Join(names, ", "))
	}
	var selected identity
	for _, item := range identities {
		selected = item
	}
	var filtered []runObservation
	for _, record := range records {
		if record.Model.ID == selected.id && record.Model.Name == selected.name {
			filtered = append(filtered, record)
		}
	}
	return selected.id, selected.name, filtered, nil
}

func buildCharacterization(reference string) (modelCharacterization, error) {
	modelID, modelName, records, err := historicalModelRecords(reference)
	if err != nil {
		return modelCharacterization{}, err
	}
	report := modelCharacterization{SchemaVersion: 1, GeneratedAt: time.Now(), ModelID: modelID, ModelName: modelName, Runs: len(records), SchemaVersions: map[int]int{}, Profiles: map[string]int{}, FailureKinds: map[string]int{}}
	categoryStats := map[string]*categoryCharacterization{}
	experiments := map[string]bool{}
	var elapsed []int64
	var speeds []float64
	for _, record := range records {
		report.SchemaVersions[record.SchemaVersion]++
		profile := record.Profile.ID
		if profile == "" {
			profile = "legacy/default"
		}
		report.Profiles[profile]++
		if record.Experiment.ID != "" {
			experiments[record.Experiment.ID] = true
		}
		if record.Input.Class == "private" || record.Exercise.Category == "freeform" || strings.HasPrefix(record.Exercise.Category, "work") {
			report.PrivateRuns++
		} else {
			report.CanonicalRuns++
		}
		if report.LatestRun.IsZero() || record.StartedAt.After(report.LatestRun) {
			report.LatestRun = record.StartedAt
		}
		if record.Machine.MemoryBytes > report.MachineMemoryBytes {
			report.MachineMemoryBytes = record.Machine.MemoryBytes
		}
		if record.Resources.RuntimeRSSBytes > 0 {
			report.RuntimeRSSSamples++
			if record.Resources.RuntimeRSSBytes > report.MaxRuntimeRSSBytes {
				report.MaxRuntimeRSSBytes = record.Resources.RuntimeRSSBytes
			}
		}
		if record.Result.Status == "succeeded" {
			elapsed = append(elapsed, record.Result.ElapsedMS)
			if record.Result.GenerationTokensSec > 0 {
				speeds = append(speeds, record.Result.GenerationTokensSec)
			}
		}
		stat := categoryStats[record.Exercise.Category]
		if stat == nil {
			stat = &categoryCharacterization{Category: record.Exercise.Category}
			categoryStats[record.Exercise.Category] = stat
		}
		stat.Runs++
		switch record.Evaluation.Status {
		case "pass":
			stat.Pass++
			stat.Scored++
		case "fail":
			stat.Fail++
			stat.Scored++
			kind := record.Evaluation.FailureKind
			if kind == "" {
				kind = "unclassified_legacy"
			}
			report.FailureKinds[kind]++
		case "pending":
			stat.Pending++
		}
	}
	report.Experiments = len(experiments)
	report.MedianElapsed = medianDuration(elapsed)
	report.MedianGeneration = medianSpeed(speeds)
	for _, stat := range categoryStats {
		stat.Coverage = evidenceCoverage(stat.Runs)
		report.Categories = append(report.Categories, *stat)
	}
	sort.Slice(report.Categories, func(i, j int) bool { return report.Categories[i].Category < report.Categories[j].Category })
	return report, nil
}

func evidenceCoverage(runs int) string {
	switch {
	case runs == 0:
		return "NONE"
	case runs < 5:
		return "EARLY"
	case runs < 15:
		return "MODERATE"
	case runs < 40:
		return "STRONG"
	default:
		return "VERY_STRONG"
	}
}

func runReport(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl report <model> [--json]")
		return 1
	}
	report, err := buildCharacterization(args[2])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	jsonOutput := len(args) >= 4 && args[3] == "--json"
	if jsonOutput {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}
	fmt.Fprintf(stdout, "LocalCTL characterization — %s\n\n", report.ModelName)
	fmt.Fprintf(stdout, "runs:        %d (%d canonical / %d private)\n", report.Runs, report.CanonicalRuns, report.PrivateRuns)
	fmt.Fprintf(stdout, "experiments: %d\n", report.Experiments)
	fmt.Fprintf(stdout, "median:      %s · %s\n", report.MedianElapsed, report.MedianGeneration)
	if report.MaxRuntimeRSSBytes > 0 {
		fmt.Fprintf(stdout, "runtime RSS: %s max observed across %d samples\n", formatBytes(report.MaxRuntimeRSSBytes), report.RuntimeRSSSamples)
		fmt.Fprintln(stdout, "             point-in-time process RSS only; not peak or total Metal/unified-memory use")
	}
	if !report.LatestRun.IsZero() {
		fmt.Fprintf(stdout, "latest:      %s\n", report.LatestRun.Format(time.RFC3339))
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Capability evidence")
	fmt.Fprintln(stdout, "CATEGORY          EVIDENCE      AUTO SCORE       RUNS  PENDING")
	for _, category := range report.Categories {
		fmt.Fprintf(stdout, "%-17s %-13s %-16s %-5d %d\n", category.Category, category.Coverage, percent(category.Pass, category.Scored), category.Runs, category.Pending)
	}
	if len(report.FailureKinds) > 0 {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Observed failure modes")
		var kinds []string
		for kind := range report.FailureKinds {
			kinds = append(kinds, kind)
		}
		sort.Strings(kinds)
		for _, kind := range kinds {
			fmt.Fprintf(stdout, "%-30s %d\n", kind, report.FailureKinds[kind])
		}
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "This is characterization, not qualification. Evidence strength reflects coverage, not a universal model grade.")
	return 0
}

func runGaps(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl gaps <model> [--json]")
		return 1
	}
	report, err := buildCharacterization(args[2])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	counts := map[string]int{}
	for _, category := range report.Categories {
		counts[category.Category] = category.Runs
	}
	type gap struct {
		Pack     string `json:"pack"`
		Coverage string `json:"coverage"`
		Runs     int    `json:"runs"`
		Why      string `json:"why"`
	}
	var gaps []gap
	for _, pack := range listPacks() {
		if pack.BaselineOnly {
			continue
		}
		total := 0
		for _, category := range pack.Categories {
			total += counts[category]
		}
		coverage := evidenceCoverage(total)
		if coverage == "STRONG" || coverage == "VERY_STRONG" {
			continue
		}
		gaps = append(gaps, gap{Pack: pack.ID, Coverage: coverage, Runs: total, Why: pack.Purpose})
	}
	sort.Slice(gaps, func(i, j int) bool {
		if gaps[i].Runs == gaps[j].Runs {
			return gaps[i].Pack < gaps[j].Pack
		}
		return gaps[i].Runs < gaps[j].Runs
	})
	if len(args) >= 4 && args[3] == "--json" {
		data, _ := json.MarshalIndent(gaps, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}
	fmt.Fprintf(stdout, "Evidence gaps — %s\n\n", report.ModelName)
	if len(gaps) == 0 {
		fmt.Fprintln(stdout, "No major pack-level coverage gaps under the current deterministic thresholds.")
		return 0
	}
	fmt.Fprintln(stdout, "PACK                       COVERAGE      RUNS   WHY IT MATTERS")
	for _, item := range gaps {
		fmt.Fprintf(stdout, "%-26s %-13s %-6d %s\n", item.Pack, item.Coverage, item.Runs, item.Why)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Highest-information next experiment: localctl pack run %s %s\n", gaps[0].Pack, report.ModelID)
	fmt.Fprintln(stdout, "This recommendation is deterministic coverage analysis; no model is interpreting the evidence.")
	return 0
}
