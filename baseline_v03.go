package main

import (
	"fmt"
	"io"
	"strings"
)

func runBaselineV03(args []string, stdout, stderr io.Writer) int {
	modelRef := ""
	profileRef := "default"
	includeExtended := false
	category := ""

	for _, arg := range args[2:] {
		switch {
		case arg == "--all":
			includeExtended = true
		case strings.HasPrefix(arg, "--category="):
			category = strings.TrimPrefix(arg, "--category=")
		case strings.HasPrefix(arg, "--profile="):
			profileRef = strings.TrimPrefix(arg, "--profile=")
		case modelRef == "":
			modelRef = arg
		default:
			fmt.Fprintf(stderr, "unexpected baseline argument: %s\n", arg)
			return 1
		}
	}

	if category != "" && !knownExerciseCategory(category) {
		fmt.Fprintf(stderr, "unknown baseline category: %s\n", category)
		fmt.Fprintf(stderr, "categories: %s\n", joinCategories())
		return 1
	}

	var model modelArtifact
	var err error
	if modelRef == "" {
		model, err = modelForExecution("")
	} else {
		model, err = resolveModel(modelRef)
	}
	if err != nil {
		fmt.Fprintf(stderr, "model resolution failed: %v\n", err)
		return 1
	}
	profile, err := resolveProfile(profileRef)
	if err != nil {
		fmt.Fprintf(stderr, "profile resolution failed: %v\n", err)
		return 1
	}

	items := exercisesForCategory(category, includeExtended)
	if len(items) == 0 {
		fmt.Fprintln(stderr, "no exercises selected")
		return 1
	}

	pack := baselineSelectionPack(category, includeExtended)
	return executeSelectionExperiment("baseline", pack, items, model, profile, stdout, stderr)
}

func baselineSelectionPack(category string, includeExtended bool) packManifest {
	if category == "" && !includeExtended {
		pack, _ := findPack("core-baseline")
		return pack
	}
	if category != "" && includeExtended {
		return packManifest{
			ID:           "category-" + category,
			Version:      "v1",
			Title:        category + " capability suite",
			Purpose:      "Characterize all built-in exercises in the " + category + " category.",
			Categories:   []string{category},
			DoesNotProve: "Capability outside this category or production readiness.",
		}
	}
	if category != "" {
		return packManifest{
			ID:           "baseline-" + category,
			Version:      "v1",
			Title:        category + " core baseline",
			Purpose:      "Run the core baseline subset for the " + category + " category.",
			Categories:   []string{category},
			DoesNotProve: "Extended category capability.",
		}
	}
	return packManifest{
		ID:           "extended-all",
		Version:      "v1",
		Title:        "Extended catalog",
		Purpose:      "Run the complete built-in LocalCTL catalog as one experiment.",
		DoesNotProve: "Production qualification or capability outside the catalog.",
	}
}

func executeSelectionExperiment(kind string, pack packManifest, items []exercise, model modelArtifact, profile profileDefinition, stdout, stderr io.Writer) int {
	if _, code := ensureRuntimeForModelProfile(model, profile, stdout, stderr); code != 0 {
		return code
	}
	session, finishSession := startSession(kind, pack.ID+" "+model.ID)
	defer finishSession()
	experiment, finishExperiment := startScopedExperiment(kind, pack.ID+"/"+pack.Version, model, profile, pack, "canonical")
	defer finishExperiment()

	fmt.Fprintf(stdout, "\nBaseline lab: %s\n", model.Name)
	fmt.Fprintf(stdout, "pack: %s/%s\nprofile: %s\nsession: %s\nexperiment: %s\n", pack.ID, pack.Version, profile.ID, session.ID, experiment.ID)
	fmt.Fprintf(stdout, "Exercises: %d\n\n", len(items))

	summary := runItemsV03(items, model, profile, stdout)
	printSetSummary(stdout, summary)
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Runtime stays ready on %s with profile %s.\n", model.Name, profile.ID)
	fmt.Fprintln(stdout, "Use 'localctl report <model>' for the accumulated historical view.")
	if summary.Errors > 0 {
		return 1
	}
	return 0
}
