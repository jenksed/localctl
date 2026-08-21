package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

func runCapability(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl capability <model> [--profile=default] [--json]")
		return 1
	}
	profileRef := "default"
	jsonOutput := false
	for _, arg := range args[3:] {
		switch {
		case strings.HasPrefix(arg, "--profile="):
			profileRef = strings.TrimPrefix(arg, "--profile=")
		case arg == "--json":
			jsonOutput = true
		default:
			fmt.Fprintf(stderr, "unexpected capability argument: %s\n", arg)
			return 1
		}
	}
	capabilities, err := buildModelCapabilityMap(args[2], profileRef, time.Now())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	snapshotID, snapshotPath, err := persistCapabilityMap(capabilities)
	if err != nil {
		fmt.Fprintf(stderr, "could not persist derived capability intelligence: %v\n", err)
		return 1
	}
	if jsonOutput {
		data, _ := json.MarshalIndent(capabilities, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}
	printCapabilityMap(stdout, capabilities)
	fmt.Fprintf(stdout, "\nderived snapshot: %s\n", snapshotID)
	fmt.Fprintf(stdout, "saved: %s\n", snapshotPath)
	fmt.Fprintln(stdout, "Inspect provenance with: localctl intelligence show <snapshot-id>")
	return 0
}

func printCapabilityMap(stdout io.Writer, capabilities modelCapabilityMap) {
	fmt.Fprintf(stdout, "LocalCTL capability map — %s\n\n", capabilities.ModelName)
	fmt.Fprintf(stdout, "rules:     %s\n", capabilities.RuleVersion)
	fmt.Fprintf(stdout, "profile:   %s\n", capabilities.ProfileID)
	fmt.Fprintf(stdout, "installed: %t\n", capabilities.Installed)
	if capabilities.ArtifactSHA != "" {
		fmt.Fprintf(stdout, "artifact:  sha256:%s\n", shorten(capabilities.ArtifactSHA, 20))
	}
	fmt.Fprintf(stdout, "machine:   %s/%s", capabilities.Machine.OS, capabilities.Machine.Architecture)
	if capabilities.Machine.Chip != "" {
		fmt.Fprintf(stdout, " · %s", capabilities.Machine.Chip)
	}
	fmt.Fprintln(stdout)
	if len(capabilities.Roles) > 0 {
		fmt.Fprintf(stdout, "candidate roles: %s\n", strings.Join(capabilities.Roles, ", "))
	} else {
		fmt.Fprintln(stdout, "candidate roles: none supported by current evidence yet")
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "PACK                       STATE               EVIDENCE      FRESHNESS   AUTO       COVERAGE  RUNS")
	for _, assessment := range capabilities.Assessments {
		fmt.Fprintf(stdout, "%-26s %-19s %-13s %-11s %-10s %-9s %d\n",
			assessment.PackID,
			assessment.State,
			assessment.Strength,
			assessment.Freshness,
			percent(assessment.Pass, assessment.Scored),
			formatPercentFloat(assessment.Coverage),
			assessment.ComparableRuns,
		)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "State means 'supported by this bounded evidence', not qualified for autonomous execution.")
	fmt.Fprintln(stdout, "Only schema-v3 canonical evidence matching the current pack/profile/machine/artifact boundary contributes.")
}

func formatPercentFloat(value float64) string {
	if value <= 0 {
		return "0.0%"
	}
	return fmt.Sprintf("%.1f%%", value*100)
}

func runRecommend(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl recommend <pack> [--profile=default] [--json]")
		return 1
	}
	pack, err := findPack(args[2])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	profileRef := "default"
	jsonOutput := false
	for _, arg := range args[3:] {
		switch {
		case strings.HasPrefix(arg, "--profile="):
			profileRef = strings.TrimPrefix(arg, "--profile=")
		case arg == "--json":
			jsonOutput = true
		default:
			fmt.Fprintf(stderr, "unexpected recommend argument: %s\n", arg)
			return 1
		}
	}
	profile, err := resolveProfile(profileRef)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	recommendations, err := buildRecommendations(pack, profile, time.Now())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if jsonOutput {
		data, _ := json.MarshalIndent(recommendations, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}
	fmt.Fprintf(stdout, "LocalCTL workload recommendation — %s/%s\n\n", pack.ID, pack.Version)
	fmt.Fprintf(stdout, "profile: %s\n", profile.ID)
	fmt.Fprintln(stdout, "basis: current schema-v3 canonical evidence matching this machine, exact artifact, profile, and pack version")
	fmt.Fprintln(stdout)
	if len(recommendations) == 0 {
		fmt.Fprintln(stdout, "No installed models have comparable evidence yet.")
		fmt.Fprintf(stdout, "Next experiment: localctl pack run %s <model> --profile=%s\n", pack.ID, profile.ID)
		return 0
	}
	fmt.Fprintln(stdout, "MODEL                        STATE               EVIDENCE      AUTO       COVERAGE  RUNS")
	for _, item := range recommendations {
		fmt.Fprintf(stdout, "%-28s %-19s %-13s %-10s %-9s %d\n", shorten(item.ModelID, 28), item.State, item.Strength, formatPercentFloat(item.PassRate), formatPercentFloat(item.Coverage), item.ComparableRuns)
	}
	fmt.Fprintln(stdout)
	if recommendations[0].Supported {
		fmt.Fprintf(stdout, "evidence leader: %s\n", recommendations[0].ModelName)
		fmt.Fprintln(stdout, "Meaning: this installed model currently has the strongest bounded evidence in this comparison set.")
	} else {
		fmt.Fprintln(stdout, "No installed model currently reaches SUPPORTED evidence for this pack.")
		fmt.Fprintf(stdout, "Highest-information next test: localctl pack run %s %s --profile=%s\n", pack.ID, recommendations[0].ModelID, profile.ID)
	}
	fmt.Fprintln(stdout, "Recommendation is advisory. It does not authorize execution or claim universal model superiority.")
	return 0
}

func runMatrix(args []string, stdout, stderr io.Writer) int {
	profileRef := "default"
	for _, arg := range args[2:] {
		if strings.HasPrefix(arg, "--profile=") {
			profileRef = strings.TrimPrefix(arg, "--profile=")
		} else {
			fmt.Fprintf(stderr, "unexpected matrix argument: %s\n", arg)
			return 1
		}
	}
	profile, err := resolveProfile(profileRef)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	models, err := discoverModels()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "LocalCTL capability matrix — profile %s\n\n", profile.ID)
	fmt.Fprintln(stdout, "MODEL                    DEV        LINUX      DOCKER     K8S        WRITING    STRUCT     REASON")
	for _, model := range models {
		capabilities, buildErr := buildModelCapabilityMap(model.ID, profile.ID, time.Now())
		if buildErr != nil {
			fmt.Fprintf(stdout, "%-24s ERROR %s\n", shorten(model.ID, 24), buildErr)
			continue
		}
		states := map[string]string{}
		for _, assessment := range capabilities.Assessments {
			states[assessment.PackID] = matrixState(assessment.State)
		}
		fmt.Fprintf(stdout, "%-24s %-10s %-10s %-10s %-10s %-10s %-10s %-10s\n",
			shorten(model.ID, 24),
			states["developer-core"],
			states["linux-investigation"],
			states["docker-investigation"],
			states["kubernetes-investigation"],
			states["writing-summarization"],
			states["structured-output"],
			states["reasoning-analysis"],
		)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Legend: STRONG / SUPPORTED / PROMISING / MIXED / WEAK / STALE / REVIEW / ?")
	fmt.Fprintln(stdout, "The matrix is a current evidence map, not an authority or a permanent model ranking.")
	return 0
}

func matrixState(state string) string {
	switch state {
	case "REVIEW_REQUIRED":
		return "REVIEW"
	case "RUNTIME_UNRELIABLE":
		return "RUNTIME"
	case "UNKNOWN":
		return "?"
	default:
		return state
	}
}

func runRequalify(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl requalify <model> [--profile=default] [--run]")
		return 1
	}
	profileRef := "default"
	execute := false
	for _, arg := range args[3:] {
		switch {
		case strings.HasPrefix(arg, "--profile="):
			profileRef = strings.TrimPrefix(arg, "--profile=")
		case arg == "--run":
			execute = true
		default:
			fmt.Fprintf(stderr, "unexpected requalify argument: %s\n", arg)
			return 1
		}
	}
	capabilities, err := buildModelCapabilityMap(args[2], profileRef, time.Now())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	type target struct {
		Assessment capabilityAssessment
		Priority   int
	}
	var targets []target
	for _, assessment := range capabilities.Assessments {
		if assessment.PackID == "core-baseline" {
			continue
		}
		priority := requalificationPriority(assessment.State, assessment.Freshness)
		if priority > 0 {
			targets = append(targets, target{Assessment: assessment, Priority: priority})
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Priority == targets[j].Priority {
			return targets[i].Assessment.PackID < targets[j].Assessment.PackID
		}
		return targets[i].Priority > targets[j].Priority
	})
	fmt.Fprintf(stdout, "LocalCTL requalification plan — %s\n\n", capabilities.ModelName)
	fmt.Fprintf(stdout, "profile: %s\n", profileRef)
	if len(targets) == 0 {
		fmt.Fprintln(stdout, "No pack currently crosses the v0.4 requalification threshold.")
		return 0
	}
	fmt.Fprintln(stdout, "PRIORITY  PACK                       STATE               FRESHNESS   NEXT COMMAND")
	for _, item := range targets {
		fmt.Fprintf(stdout, "%-9d %-26s %-19s %-11s localctl pack run %s %s --profile=%s\n",
			item.Priority,
			item.Assessment.PackID,
			item.Assessment.State,
			item.Assessment.Freshness,
			item.Assessment.PackID,
			capabilities.ModelID,
			profileRef,
		)
	}
	if !execute {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Plan only. Add --run to execute the single highest-priority pack.")
		return 0
	}
	pack, err := findPack(targets[0].Assessment.PackID)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "\nExecuting highest-priority requalification target: %s/%s\n", pack.ID, pack.Version)
	return executePackRun(pack, capabilities.ModelID, profileRef, stdout, stderr)
}
