package main

import (
	"fmt"
	"io"
	"strings"
)

func runHeadToHead(args []string, stdout, stderr io.Writer) int {
	if len(args) < 4 {
		fmt.Fprintln(stderr, "usage: localctl headtohead <model-a> <model-b> [--pack=core-baseline] [--profile=default]")
		return 1
	}
	modelA, err := resolveModel(args[2])
	if err != nil {
		fmt.Fprintf(stderr, "model A: %v\n", err)
		return 1
	}
	modelB, err := resolveModel(args[3])
	if err != nil {
		fmt.Fprintf(stderr, "model B: %v\n", err)
		return 1
	}
	packRef := "core-baseline"
	profileRef := "default"
	for _, arg := range args[4:] {
		switch {
		case strings.HasPrefix(arg, "--pack="):
			packRef = strings.TrimPrefix(arg, "--pack=")
		case strings.HasPrefix(arg, "--profile="):
			profileRef = strings.TrimPrefix(arg, "--profile=")
		default:
			fmt.Fprintf(stderr, "unexpected headtohead argument: %s\n", arg)
			return 1
		}
	}
	pack, err := findPack(packRef)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	profile, err := resolveProfile(profileRef)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	items := packExercises(pack)
	if len(items) == 0 {
		fmt.Fprintf(stderr, "pack %s has no exercises\n", pack.ID)
		return 1
	}

	session, finishSession := startSession("headtohead", modelA.ID+" vs "+modelB.ID+" · "+pack.ID)
	defer finishSession()

	fmt.Fprintln(stdout, "LocalCTL head-to-head")
	fmt.Fprintf(stdout, "session: %s\n", session.ID)
	fmt.Fprintf(stdout, "pack: %s/%s\n", pack.ID, pack.Version)
	fmt.Fprintf(stdout, "profile: %s\n", profile.ID)
	fmt.Fprintf(stdout, "models: %s vs %s\n\n", modelA.Name, modelB.Name)

	fmt.Fprintf(stdout, "===== MODEL A: %s =====\n", modelA.Name)
	a, code := runHeadToHeadSide(modelA, profile, pack, items, stdout, stderr)
	if code != 0 {
		return code
	}
	fmt.Fprintf(stdout, "\n===== MODEL B: %s =====\n", modelB.Name)
	b, code := runHeadToHeadSide(modelB, profile, pack, items, stdout, stderr)
	if code != 0 {
		return code
	}

	fmt.Fprintln(stdout, "\n===== CONTROLLED COMPARISON =====")
	fmt.Fprintf(stdout, "%-24s %-16s %-16s\n", "", shorten(modelA.ID, 16), shorten(modelB.ID, 16))
	fmt.Fprintf(stdout, "%-24s %-16s %-16s\n", "auto pass rate", percent(a.Pass, a.Pass+a.Fail), percent(b.Pass, b.Pass+b.Fail))
	fmt.Fprintf(stdout, "%-24s %-16d %-16d\n", "manual pending", a.Pending, b.Pending)
	fmt.Fprintf(stdout, "%-24s %-16s %-16s\n", "median elapsed", observationMedianElapsed(a.Records), observationMedianElapsed(b.Records))
	fmt.Fprintf(stdout, "%-24s %-16s %-16s\n", "median generation", observationMedianSpeed(a.Records), observationMedianSpeed(b.Records))
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Comparable by construction: same LocalCTL session, machine, pack version, and requested profile.")
	fmt.Fprintln(stdout, "This still characterizes only the tested workload; it is not a universal model ranking.")
	return 0
}

func runHeadToHeadSide(model modelArtifact, profile profileDefinition, pack packManifest, items []exercise, stdout, stderr io.Writer) (experimentSetSummary, int) {
	if _, code := ensureRuntimeForModelProfile(model, profile, stdout, stderr); code != 0 {
		return experimentSetSummary{}, code
	}
	experiment, finishExperiment := startScopedExperiment("headtohead", pack.ID+"/"+pack.Version+" · "+model.ID, model, profile, pack, "canonical")
	defer finishExperiment()
	fmt.Fprintf(stdout, "experiment: %s\n", experiment.ID)
	summary := runItemsV03(items, model, profile, stdout)
	printSetSummary(stdout, summary)
	if summary.Errors > 0 {
		return summary, 1
	}
	return summary, 0
}

func observationMedianElapsed(records []runObservation) string {
	var values []int64
	for _, record := range records {
		if record.Result.Status == "succeeded" {
			values = append(values, record.Result.ElapsedMS)
		}
	}
	return medianDuration(values)
}

func observationMedianSpeed(records []runObservation) string {
	var values []float64
	for _, record := range records {
		if record.Result.GenerationTokensSec > 0 {
			values = append(values, record.Result.GenerationTokensSec)
		}
	}
	return medianSpeed(values)
}
