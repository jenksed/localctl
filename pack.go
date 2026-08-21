package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type packManifest struct {
	ID           string   `json:"id"`
	Version      string   `json:"version"`
	Title        string   `json:"title"`
	Purpose      string   `json:"purpose"`
	Categories   []string `json:"categories,omitempty"`
	BaselineOnly bool     `json:"baseline_only,omitempty"`
	DoesNotProve string   `json:"does_not_prove"`
}

var capabilityPacks = []packManifest{
	{ID: "core-baseline", Version: "v1", Title: "Core baseline", Purpose: "Fast cross-capability smoke test for instruction, structure, coding, reasoning, extraction, and analysis.", BaselineOnly: true, DoesNotProve: "General model quality or production readiness."},
	{ID: "developer-core", Version: "v1", Title: "Developer workflow", Purpose: "Coding semantics plus commit, diff, PR, regression-test, and bug-triage work.", Categories: []string{"coding", "developer"}, DoesNotProve: "Autonomous code-change safety."},
	{ID: "linux-investigation", Version: "v1", Title: "Linux investigation", Purpose: "Linux process, filesystem, networking, resource, service, and incident reasoning.", Categories: []string{"linux"}, DoesNotProve: "Safe autonomous production remediation."},
	{ID: "docker-investigation", Version: "v1", Title: "Docker investigation", Purpose: "Container lifecycle, image, storage, networking, build, and troubleshooting reasoning.", Categories: []string{"docker"}, DoesNotProve: "Safe autonomous container mutation."},
	{ID: "kubernetes-investigation", Version: "v1", Title: "Kubernetes investigation", Purpose: "Kubernetes workload, service, scheduling, probes, storage, RBAC, networking, and rollout investigation.", Categories: []string{"kubernetes"}, DoesNotProve: "Safe autonomous cluster remediation."},
	{ID: "writing-summarization", Version: "v1", Title: "Writing and summarization", Purpose: "Faithful technical summarization, concise rewriting, explanations, and user-facing developer writing.", Categories: []string{"writing", "summarization"}, DoesNotProve: "Factual correctness outside the supplied source."},
	{ID: "structured-output", Version: "v1", Title: "Structured and extraction", Purpose: "Strict output contracts, JSON, extraction, and bounded classification.", Categories: []string{"instruction", "structured", "extraction"}, DoesNotProve: "Open-ended reasoning quality."},
	{ID: "reasoning-analysis", Version: "v1", Title: "Reasoning and analysis", Purpose: "Constraint reasoning, uncertainty, grounding, contradictions, and evidence distinctions.", Categories: []string{"reasoning", "analysis"}, DoesNotProve: "Reliability on unseen complex domains."},
}

func listPacks() []packManifest {
	packs := append([]packManifest(nil), capabilityPacks...)
	sort.Slice(packs, func(i, j int) bool { return packs[i].ID < packs[j].ID })
	return packs
}

func findPack(reference string) (packManifest, error) {
	if reference == "" {
		return packManifest{}, nil
	}
	needle := strings.ToLower(reference)
	var matches []packManifest
	for _, pack := range capabilityPacks {
		if strings.EqualFold(pack.ID, reference) {
			return pack, nil
		}
		if strings.Contains(strings.ToLower(pack.ID), needle) || strings.Contains(strings.ToLower(pack.Title), needle) {
			matches = append(matches, pack)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return packManifest{}, fmt.Errorf("pack reference %q is ambiguous", reference)
	}
	return packManifest{}, fmt.Errorf("pack %q not found", reference)
}

func packExercises(pack packManifest) []exercise {
	var result []exercise
	for _, item := range allExerciseCatalog() {
		if pack.BaselineOnly {
			if item.Baseline {
				result = append(result, item)
			}
			continue
		}
		for _, category := range pack.Categories {
			if item.Category == category {
				result = append(result, item)
				break
			}
		}
	}
	return result
}

func inferredPackForExercise(item exercise) packManifest {
	for _, pack := range capabilityPacks {
		if pack.BaselineOnly && item.Baseline {
			return pack
		}
		for _, category := range pack.Categories {
			if item.Category == category {
				return pack
			}
		}
	}
	return packManifest{ID: "uncategorized", Version: "v1", Title: "Uncategorized run", Purpose: "Run outside a canonical capability pack.", DoesNotProve: "Any broader capability."}
}

func runPacks(stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, "PACK                       VERSION   EXERCISES   PURPOSE")
	for _, pack := range listPacks() {
		fmt.Fprintf(stdout, "%-26s %-9s %-11d %s\n", pack.ID, pack.Version, len(packExercises(pack)), pack.Purpose)
	}
	return 0
}

func runPack(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl pack <show|run> ...")
		return 1
	}
	switch args[2] {
	case "show":
		if len(args) < 4 {
			fmt.Fprintln(stderr, "usage: localctl pack show <pack>")
			return 1
		}
		pack, err := findPack(args[3])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "%s/%s — %s\n", pack.ID, pack.Version, pack.Title)
		fmt.Fprintf(stdout, "purpose: %s\n", pack.Purpose)
		fmt.Fprintf(stdout, "exercises: %d\n", len(packExercises(pack)))
		fmt.Fprintf(stdout, "does not prove: %s\n", pack.DoesNotProve)
		if len(pack.Categories) > 0 {
			fmt.Fprintf(stdout, "categories: %s\n", strings.Join(pack.Categories, ", "))
		}
		return 0
	case "run":
		if len(args) < 5 {
			fmt.Fprintln(stderr, "usage: localctl pack run <pack> <model> [--profile=default]")
			return 1
		}
		pack, err := findPack(args[3])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		profileRef := "default"
		for _, arg := range args[5:] {
			if strings.HasPrefix(arg, "--profile=") {
				profileRef = strings.TrimPrefix(arg, "--profile=")
			} else {
				fmt.Fprintf(stderr, "unexpected pack argument: %s\n", arg)
				return 1
			}
		}
		return executePackRun(pack, args[4], profileRef, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown pack command: %s\n", args[2])
		return 1
	}
}
