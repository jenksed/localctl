package main

import (
	"fmt"
	"io"
	"sort"
)

type historicalInsightStats struct {
	Runs           int
	Succeeded      int
	Errors         int
	Pass           int
	Fail           int
	Pending        int
	JudgedGood     int
	JudgedPartial  int
	JudgedBad      int
	ElapsedMS      []int64
	Speeds         []float64
	Categories     map[string][2]int
	FailureKinds   map[string]int
	EmptyOutputs   int
	SchemaVersions map[int]int
}

func runInsights(args []string, stdout, stderr io.Writer) int {
	modelRef := ""
	if len(args) >= 3 {
		modelRef = args[2]
	}

	var selected *modelArtifact
	if modelRef != "" {
		model, err := resolveModel(modelRef)
		if err != nil {
			fmt.Fprintf(stderr, "model resolution failed: %v\n", err)
			return 1
		}
		selected = &model
	}

	records, runDirs, err := scanObservationFiles()
	if err != nil {
		fmt.Fprintf(stderr, "could not read evidence: %v\n", err)
		return 1
	}

	stats := historicalInsightStats{
		Categories:     map[string][2]int{},
		FailureKinds:   map[string]int{},
		SchemaVersions: map[int]int{},
	}

	for _, record := range records {
		if selected != nil && record.Model.Path != selected.Path && record.Model.ID != selected.ID && record.Model.Name != selected.Name {
			continue
		}
		stats.Runs++
		stats.SchemaVersions[record.SchemaVersion]++
		if record.Result.Status != "succeeded" {
			stats.Errors++
			continue
		}
		stats.Succeeded++
		stats.ElapsedMS = append(stats.ElapsedMS, record.Result.ElapsedMS)
		if record.Result.GenerationTokensSec > 0 {
			stats.Speeds = append(stats.Speeds, record.Result.GenerationTokensSec)
		}
		if record.Result.EmptyVisibleOutput {
			stats.EmptyOutputs++
		}

		switch record.Evaluation.Status {
		case "pass":
			stats.Pass++
			value := stats.Categories[record.Exercise.Category]
			value[0]++
			value[1]++
			stats.Categories[record.Exercise.Category] = value
		case "fail":
			stats.Fail++
			value := stats.Categories[record.Exercise.Category]
			value[1]++
			stats.Categories[record.Exercise.Category] = value
			kind := record.Evaluation.FailureKind
			if kind == "" {
				kind = "unclassified_legacy"
			}
			stats.FailureKinds[kind]++
		case "pending":
			stats.Pending++
		}

		if runDir := runDirs[record.RunID]; runDir != "" {
			if judgment, judgmentErr := loadJudgment(runDir); judgmentErr == nil && judgment != nil {
				switch judgment.Verdict {
				case "good":
					stats.JudgedGood++
				case "partial":
					stats.JudgedPartial++
				case "bad":
					stats.JudgedBad++
				}
			}
		}
	}

	fmt.Fprintln(stdout, "LocalCTL historical insights")
	if selected != nil {
		fmt.Fprintf(stdout, "model: %s\n", selected.Name)
	} else {
		fmt.Fprintln(stdout, "scope: all saved models")
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "runs:                 %d\n", stats.Runs)
	fmt.Fprintf(stdout, "successful inference: %d\n", stats.Succeeded)
	fmt.Fprintf(stdout, "inference errors:      %d\n", stats.Errors)
	fmt.Fprintf(stdout, "auto pass:             %d\n", stats.Pass)
	fmt.Fprintf(stdout, "auto fail:             %d\n", stats.Fail)
	fmt.Fprintf(stdout, "manual pending:        %d\n", stats.Pending)
	fmt.Fprintf(stdout, "empty visible output:  %d\n", stats.EmptyOutputs)
	fmt.Fprintf(stdout, "median elapsed:        %s\n", medianDuration(stats.ElapsedMS))
	fmt.Fprintf(stdout, "median generation:     %s\n", medianSpeed(stats.Speeds))
	if stats.Pass+stats.Fail > 0 {
		fmt.Fprintf(stdout, "auto-scored pass rate: %s\n", percent(stats.Pass, stats.Pass+stats.Fail))
	}

	if stats.JudgedGood+stats.JudgedPartial+stats.JudgedBad > 0 {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Human judgments")
		fmt.Fprintf(stdout, "good:    %d\n", stats.JudgedGood)
		fmt.Fprintf(stdout, "partial: %d\n", stats.JudgedPartial)
		fmt.Fprintf(stdout, "bad:     %d\n", stats.JudgedBad)
	}

	var categories []string
	for category, score := range stats.Categories {
		if score[1] > 0 {
			categories = append(categories, category)
		}
	}
	sort.Strings(categories)
	if len(categories) > 0 {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Auto-scored capability by category")
		for _, category := range categories {
			score := stats.Categories[category]
			fmt.Fprintf(stdout, "%-16s %4d/%-4d %s\n", category, score[0], score[1], percent(score[0], score[1]))
		}
	}

	var failureKinds []string
	for kind := range stats.FailureKinds {
		failureKinds = append(failureKinds, kind)
	}
	sort.Slice(failureKinds, func(i, j int) bool {
		if stats.FailureKinds[failureKinds[i]] == stats.FailureKinds[failureKinds[j]] {
			return failureKinds[i] < failureKinds[j]
		}
		return stats.FailureKinds[failureKinds[i]] > stats.FailureKinds[failureKinds[j]]
	})
	if len(failureKinds) > 0 {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Failure modes")
		for _, kind := range failureKinds {
			fmt.Fprintf(stdout, "%-28s %d\n", kind, stats.FailureKinds[kind])
		}
	}

	var versions []int
	for version := range stats.SchemaVersions {
		versions = append(versions, version)
	}
	sort.Ints(versions)
	if len(versions) > 0 {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Evidence schema mix")
		for _, version := range versions {
			fmt.Fprintf(stdout, "v%d: %d runs\n", version, stats.SchemaVersions[version])
		}
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Interpretation: these are historical observations for this machine/runtime corpus, not a universal model ranking.")
	return 0
}
