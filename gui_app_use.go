package main

import (
	"context"
	"fmt"
)

var useWorkloads = []struct{ ID, Title, PackID string }{
	{"coding", "Coding", "developer-core"},
	{"structured-output", "Structured output", "structured-output"},
	{"reasoning", "Reasoning", "reasoning-analysis"},
	{"linux", "Linux troubleshooting", "linux-investigation"},
	{"docker", "Docker troubleshooting", "docker-investigation"},
	{"kubernetes", "Kubernetes troubleshooting", "kubernetes-investigation"},
	{"writing", "Writing", "writing-summarization"},
}

func (a *localApplication) Recommend(ctx context.Context, workload, profileRef string) (useRecommendation, error) {
	if profileRef == "" {
		profileRef = "default"
	}
	var definition *struct{ ID, Title, PackID string }
	for i := range useWorkloads {
		if useWorkloads[i].ID == workload || useWorkloads[i].PackID == workload {
			definition = &useWorkloads[i]
			break
		}
	}
	if definition == nil {
		return useRecommendation{}, fmt.Errorf("workload %q not found", workload)
	}
	profile, err := resolveProfile(profileRef)
	if err != nil {
		return useRecommendation{}, err
	}
	territory, err := a.GetTerritory(ctx, profileRef)
	if err != nil {
		return useRecommendation{}, err
	}
	result := useRecommendation{Workload: definition.ID, Title: definition.Title, PackID: definition.PackID, State: "UNKNOWN", Profile: profile}
	for _, cell := range territory.Cells {
		if cell.PackID != definition.PackID {
			continue
		}
		result.State = cell.State
		result.Leader = cell.EvidenceLeader
		result.EvidenceRuns = cell.ComparableRuns
		result.SourceRunIDs = append([]string(nil), cell.SourceRunIDs...)
		result.Limitations = append([]string(nil), cell.Limitations...)
		result.Supported = cell.EvidenceLeader != nil && (cell.State == "SUPPORTED" || cell.State == "STRONG") && cell.Freshness == "CURRENT"
		break
	}
	if result.Supported && result.Leader != nil {
		model, resolveErr := resolveModel(result.Leader.ModelID)
		if resolveErr != nil {
			return useRecommendation{}, resolveErr
		}
		result.Preview = map[string]interface{}{
			"kind":           "openai-compatible-local",
			"endpoint":       runtimeURL + "/v1",
			"model_id":       model.ID,
			"model_path":     model.Path,
			"profile":        profile,
			"launch_preview": fmt.Sprintf("localctl runtime start %s --profile=%s", model.ID, profile.ID),
			"authority":      "preview_only",
		}
	}
	return result, nil
}

func (a *localApplication) ListUseRecommendations(ctx context.Context, profileRef string) ([]useRecommendation, error) {
	var result []useRecommendation
	for _, workload := range useWorkloads {
		item, err := a.Recommend(ctx, workload.ID, profileRef)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (a *localApplication) Requalify(ctx context.Context, modelRef, profileRef string) ([]missionRequirement, error) {
	if profileRef == "" {
		profileRef = "default"
	}
	capabilities, err := buildModelCapabilityMap(modelRef, profileRef, a.now())
	if err != nil {
		return nil, err
	}
	var result []missionRequirement
	for _, assessment := range capabilities.Assessments {
		priority := requalificationPriority(assessment.State, assessment.Freshness)
		if priority == 0 {
			continue
		}
		result = append(result, missionRequirement{ID: assessment.PackID, Title: assessment.PackTitle, PackID: assessment.PackID, State: assessment.State, Freshness: assessment.Freshness, Runnable: true, Reason: requirementReason(assessment)})
	}
	return result, nil
}
