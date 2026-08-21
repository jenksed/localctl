package main

import (
	"fmt"
	"io"
)

func runSuite(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl suite <category> [model]")
		fmt.Fprintf(stderr, "categories: %s\n", joinCategories())
		return 1
	}

	category := args[2]
	if !knownExerciseCategory(category) {
		fmt.Fprintf(stderr, "unknown suite category: %s\n", category)
		fmt.Fprintf(stderr, "categories: %s\n", joinCategories())
		return 1
	}

	baselineArgs := []string{"localctl", "baseline"}
	if len(args) >= 4 {
		baselineArgs = append(baselineArgs, args[3])
	}
	baselineArgs = append(baselineArgs, "--all", "--category="+category)
	return runBaseline(baselineArgs, stdout, stderr)
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
	result := categories[0]
	for _, category := range categories[1:] {
		result += ", " + category
	}
	return result
}
