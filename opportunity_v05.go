package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

type learningOpportunity struct {
	Kind        string `json:"kind"`
	Question    string `json:"question"`
	Why         string `json:"why"`
	Status      string `json:"status"`
	Priority    int    `json:"priority"`
	ModelID     string `json:"model_id,omitempty"`
	PackID      string `json:"pack_id,omitempty"`
	Repository  string `json:"repository,omitempty"`
	NextCommand string `json:"next_command"`
	Source      string `json:"source"`
}

func buildLocalLearningOpportunities(models []modelArtifact, profileID string, now time.Time) []learningOpportunity {
	var opportunities []learningOpportunity
	for _, model := range models {
		capabilities, err := buildModelCapabilityMap(model.ID, profileID, now)
		if err != nil {
			continue
		}
		var best *capabilityAssessment
		bestPriority := 0
		for i := range capabilities.Assessments {
			assessment := capabilities.Assessments[i]
			if assessment.PackID == "core-baseline" {
				continue
			}
			priority := requalificationPriority(assessment.State, assessment.Freshness)
			if priority > bestPriority {
				copy := assessment
				best = &copy
				bestPriority = priority
			}
		}
		if best == nil {
			continue
		}
		question := fmt.Sprintf("What can %s demonstrate on %s now?", capabilities.ModelName, best.PackID)
		why := fmt.Sprintf("local evidence is %s with %s freshness", best.State, best.Freshness)
		if best.State == "UNKNOWN" {
			why = "this installed model has no comparable canonical evidence for this workload yet"
		}
		if best.Freshness == "STALE" {
			why = "the newest comparable evidence is stale under the current v0.4 rule"
		}
		opportunities = append(opportunities, learningOpportunity{
			Kind:        "local-evidence",
			Question:    question,
			Why:         why,
			Status:      best.State,
			Priority:    300 + bestPriority,
			ModelID:     model.ID,
			PackID:      best.PackID,
			NextCommand: fmt.Sprintf("localctl pack run %s %s --profile=%s", best.PackID, model.ID, profileID),
			Source:      "local capability map / deterministic requalification rules",
		})
	}
	return opportunities
}

func ecosystemOpportunityPriority(candidate radarCandidate, now time.Time) int {
	priority := 250
	matches := len(candidate.UseMatches)
	if matches > 3 {
		matches = 3
	}
	priority += matches * 20
	if candidate.Fit == "COMFORTABLE" {
		priority += 5
	}
	if modified, err := time.Parse(time.RFC3339, candidate.LastModified); err == nil {
		days := int(now.Sub(modified).Hours() / 24)
		switch {
		case days <= 7:
			priority += 15
		case days <= 30:
			priority += 10
		case days <= 90:
			priority += 5
		}
	}
	return priority
}

func buildRadarLearningOpportunities(candidates []radarCandidate, now time.Time) []learningOpportunity {
	var opportunities []learningOpportunity
	for _, candidate := range candidates {
		why := candidate.Why
		if candidate.ParametersB > 0 {
			why += fmt.Sprintf("; parsed size class %.1fB screens as %s", candidate.ParametersB, strings.ToLower(candidate.Fit))
		}
		opportunities = append(opportunities, learningOpportunity{
			Kind:        "ecosystem-candidate",
			Question:    fmt.Sprintf("Is %s useful on this machine?", candidate.Repository),
			Why:         why,
			Status:      "UNTESTED_LOCAL",
			Priority:    ecosystemOpportunityPriority(candidate, now),
			Repository:  candidate.Repository,
			NextCommand: "localctl explore select " + candidate.Repository,
			Source:      candidate.Source,
		})
	}
	return opportunities
}

func sortLearningOpportunities(opportunities []learningOpportunity) {
	sort.SliceStable(opportunities, func(i, j int) bool {
		if opportunities[i].Priority != opportunities[j].Priority {
			return opportunities[i].Priority > opportunities[j].Priority
		}
		if opportunities[i].Kind != opportunities[j].Kind {
			return opportunities[i].Kind < opportunities[j].Kind
		}
		return opportunities[i].Question < opportunities[j].Question
	})
}

func runNext(args []string, stdout, stderr io.Writer) int {
	offline := false
	jsonOutput := false
	limit := 5
	profileID := "default"
	for _, arg := range args[2:] {
		switch {
		case arg == "--offline":
			offline = true
		case arg == "--json":
			jsonOutput = true
		case strings.HasPrefix(arg, "--limit="):
			value, err := strconv.Atoi(strings.TrimPrefix(arg, "--limit="))
			if err != nil || value < 1 || value > 25 {
				fmt.Fprintln(stderr, "--limit must be between 1 and 25")
				return 1
			}
			limit = value
		case strings.HasPrefix(arg, "--profile="):
			profileID = strings.TrimPrefix(arg, "--profile=")
		default:
			fmt.Fprintf(stderr, "unexpected next argument: %s\n", arg)
			return 1
		}
	}
	if _, err := resolveProfile(profileID); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	models, err := discoverModels()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	now := time.Now()
	opportunities := buildLocalLearningOpportunities(models, profileID, now)
	criteria, criteriaErr := loadRadarCriteria()
	if criteriaErr != nil {
		fmt.Fprintf(stderr, "radar criteria unavailable: %v\n", criteriaErr)
	}
	machine := machineFingerprint()
	radarStatus := "offline"
	if !offline && criteriaErr == nil {
		results, scanErr := fetchHFRadarResults(nil)
		if scanErr != nil {
			fmt.Fprintf(stderr, "Hugging Face radar unavailable; continuing with local evidence: %v\n", scanErr)
			radarStatus = "unavailable"
		} else {
			candidates := rankRadarResults(results, criteria, machine, models, now)
			if len(candidates) > 5 {
				candidates = candidates[:5]
			}
			opportunities = append(opportunities, buildRadarLearningOpportunities(candidates, now)...)
			radarStatus = "live"
		}
	}
	sortLearningOpportunities(opportunities)
	if len(opportunities) > limit {
		opportunities = opportunities[:limit]
	}
	if jsonOutput {
		payload := struct {
			GeneratedAt   time.Time                `json:"generated_at"`
			Machine       machineFingerprintRecord `json:"machine"`
			ProfileID     string                   `json:"profile_id"`
			RadarStatus   string                   `json:"radar_status"`
			RadarCriteria radarCriteria            `json:"radar_criteria"`
			Opportunities []learningOpportunity    `json:"opportunities"`
		}{now, machine, profileID, radarStatus, criteria, opportunities}
		data, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}
	fmt.Fprintln(stdout, "LOCALCTL — WHAT IS WORTH LEARNING NEXT?")
	fmt.Fprintf(stdout, "profile: %s · installed models: %d · ecosystem radar: %s\n", profileID, len(models), radarStatus)
	fmt.Fprintln(stdout)
	if len(opportunities) == 0 {
		fmt.Fprintln(stdout, "No bounded next opportunity was found from current local evidence and radar criteria.")
		fmt.Fprintln(stdout, "Try: localctl radar criteria")
		return 0
	}
	for i, opportunity := range opportunities {
		fmt.Fprintf(stdout, "%d. %s\n", i+1, opportunity.Question)
		fmt.Fprintf(stdout, "   kind:   %s\n", opportunity.Kind)
		fmt.Fprintf(stdout, "   status: %s\n", opportunity.Status)
		fmt.Fprintf(stdout, "   why:    %s\n", opportunity.Why)
		fmt.Fprintf(stdout, "   next:   %s\n", opportunity.NextCommand)
		fmt.Fprintf(stdout, "   source: %s\n", opportunity.Source)
		fmt.Fprintln(stdout)
	}
	fmt.Fprintln(stdout, "External model metadata can only create a test opportunity. A model becomes recommendable only after local evidence exists.")
	return 0
}
