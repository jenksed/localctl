package main

import (
	"fmt"
	"io"
	"strings"
)

func runSuite(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl suite <category> [model] [--profile=default]")
		fmt.Fprintf(stderr, "categories: %s\n", joinCategories())
		return 1
	}

	category := args[2]
	if !knownExerciseCategory(category) {
		fmt.Fprintf(stderr, "unknown suite category: %s\n", category)
		fmt.Fprintf(stderr, "categories: %s\n", joinCategories())
		return 1
	}

	modelRef := ""
	profileRef := "default"
	for _, arg := range args[3:] {
		switch {
		case strings.HasPrefix(arg, "--profile="):
			profileRef = strings.TrimPrefix(arg, "--profile=")
		case modelRef == "":
			modelRef = arg
		default:
			fmt.Fprintf(stderr, "unexpected suite argument: %s\n", arg)
			return 1
		}
	}
	if modelRef == "" {
		model, err := modelForExecution("")
		if err != nil {
			fmt.Fprintf(stderr, "model resolution failed: %v\n", err)
			return 1
		}
		modelRef = model.ID
	}

	pack := packManifest{
		ID:           "category-" + category,
		Version:      "v1",
		Title:        strings.ToUpper(category[:1]) + category[1:] + " capability suite",
		Purpose:      "Characterize the " + category + " exercise category as one comparable experiment.",
		Categories:   []string{category},
		DoesNotProve: "Capability outside this bounded category or production readiness.",
	}
	return executePackRun(pack, modelRef, profileRef, stdout, stderr)
}

func knownExerciseCategory(category string) bool {
	for _, candidate := range exerciseCategories() {
		if candidate == category {
			return true
		}
	}
	return false
}

func joinCategories() string {
	categories := exerciseCategories()
	if len(categories) == 0 {
		return "none"
	}
	return strings.Join(categories, ", ")
}
