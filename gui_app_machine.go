package main

import (
	"context"
	"sort"
	"strings"
)

func (a *localApplication) InspectMachine(ctx context.Context) (machineInspection, error) {
	if err := ctx.Err(); err != nil {
		return machineInspection{}, err
	}
	models, err := discoverModels()
	if err != nil {
		return machineInspection{}, err
	}
	records, err := listObservations()
	if err != nil {
		return machineInspection{}, err
	}
	projection := machineInspection{Machine: machineFingerprint(), Models: len(models), EvidenceRuns: len(records), LocalCTL: localctlFingerprint()}
	projection.Runtime = currentRuntimeProjection()
	projection.EvidenceRoot, _ = evidenceRoot()
	projection.IntelligenceRoot, _ = intelligenceRoot()
	return projection, nil
}

func currentRuntimeProjection() runtimeProjection {
	projection := runtimeProjection{State: "stopped", URL: runtimeURL}
	state, err := readRuntimeState()
	if err != nil {
		return projection
	}
	projection.State = "recorded"
	projection.ModelID = state.ModelID
	projection.ProfileID = state.ProfileID
	projection.PID = state.PID
	var out, errOut strings.Builder
	if runtimeStatus(runtimeURL, &out, &errOut) == 0 {
		projection.State = "ready"
		projection.Ready = true
	}
	return projection
}

func observationCountsByModel(records []runObservation) map[string][]runObservation {
	result := map[string][]runObservation{}
	for _, record := range records {
		keys := []string{record.Model.ID, record.Model.Path, record.Model.Name}
		for _, key := range keys {
			if key != "" {
				result[key] = append(result[key], record)
			}
		}
	}
	return result
}

func modelRecords(model modelArtifact, byModel map[string][]runObservation) []runObservation {
	if records := byModel[model.Path]; len(records) > 0 {
		return records
	}
	if records := byModel[model.ID]; len(records) > 0 {
		return records
	}
	return byModel[model.Name]
}

func (a *localApplication) ListModels(ctx context.Context, profileRef string) ([]modelProjection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if profileRef == "" {
		profileRef = "default"
	}
	models, err := discoverModels()
	if err != nil {
		return nil, err
	}
	records, err := listObservations()
	if err != nil {
		return nil, err
	}
	byModel := observationCountsByModel(records)
	explore, _ := loadExploreState()
	runtime := currentRuntimeProjection()
	result := make([]modelProjection, 0, len(models))
	for _, model := range models {
		matching := modelRecords(model, byModel)
		item := modelProjection{ID: model.ID, Name: model.Name, Path: model.Path, SizeBytes: model.Size, Size: formatBytes(model.Size), Quantization: quantizationFromName(model.Name), Installed: true, SavedRuns: len(matching), Tested: len(matching) > 0, Freshness: "UNKNOWN", CapabilityStates: map[string]string{}, ActiveRuntime: runtime.Ready && runtime.ModelID == model.ID}
		if len(matching) > 0 {
			latest := matching[0].StartedAt
			for _, record := range matching[1:] {
				if record.StartedAt.After(latest) {
					latest = record.StartedAt
				}
			}
			item.LatestRun = &latest
			item.Freshness, _ = capabilityFreshness(a.now(), latest)
			if capabilities, buildErr := buildModelCapabilityMap(model.ID, profileRef, a.now()); buildErr == nil {
				for _, assessment := range capabilities.Assessments {
					item.CapabilityStates[assessment.PackID] = assessment.State
					if (assessment.State == "SUPPORTED" || assessment.State == "STRONG") && assessment.Freshness == "CURRENT" {
						item.Demonstrated = true
					}
				}
			}
		}
		for _, selection := range explore.Selected {
			if candidateInstalled(selection.Candidate, []modelArtifact{model}) {
				item.SelectedCandidate = true
				break
			}
		}
		result = append(result, item)
	}
	return result, nil
}

type capabilityCandidate struct {
	model      modelArtifact
	assessment capabilityAssessment
}

func sortCapabilityCandidates(items []capabilityCandidate) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].assessment, items[j].assessment
		if recommendationStateRank(a.State) != recommendationStateRank(b.State) {
			return recommendationStateRank(a.State) > recommendationStateRank(b.State)
		}
		if recommendationStrengthRank(a.Strength) != recommendationStrengthRank(b.Strength) {
			return recommendationStrengthRank(a.Strength) > recommendationStrengthRank(b.Strength)
		}
		if a.PassRate != b.PassRate {
			return a.PassRate > b.PassRate
		}
		if a.Coverage != b.Coverage {
			return a.Coverage > b.Coverage
		}
		return items[i].model.Name < items[j].model.Name
	})
}

func (a *localApplication) capabilityMapsForTestedModels(ctx context.Context, profileRef string) ([]modelArtifact, map[string]modelCapabilityMap, int, error) {
	models, err := discoverModels()
	if err != nil {
		return nil, nil, 0, err
	}
	records, err := listObservations()
	if err != nil {
		return nil, nil, 0, err
	}
	byModel := observationCountsByModel(records)
	maps := map[string]modelCapabilityMap{}
	for _, model := range models {
		if err := ctx.Err(); err != nil {
			return nil, nil, 0, err
		}
		if len(modelRecords(model, byModel)) == 0 {
			continue
		}
		capabilities, buildErr := buildModelCapabilityMap(model.ID, profileRef, a.now())
		if buildErr == nil {
			maps[model.ID] = capabilities
		}
	}
	return models, maps, len(records), nil
}

func (a *localApplication) GetTerritory(ctx context.Context, profileRef string) (territoryProjection, error) {
	if profileRef == "" {
		profileRef = "default"
	}
	models, maps, evidenceRuns, err := a.capabilityMapsForTestedModels(ctx, profileRef)
	if err != nil {
		return territoryProjection{}, err
	}
	projection := territoryProjection{GeneratedAt: a.now(), ProfileID: profileRef, Machine: machineFingerprint(), EvidenceRuns: evidenceRuns}
	for _, definition := range territoryDefinitions {
		cell := territoryCell{ID: definition.ID, Title: definition.Title, Question: definition.Question, PackID: definition.PackID, State: "UNKNOWN", Strength: "NONE", Freshness: "UNKNOWN", Limitations: append([]string(nil), definition.Limitations...), InvestigationReady: definition.PackID != ""}
		if definition.PackID == "" {
			projection.Cells = append(projection.Cells, cell)
			continue
		}
		var candidates []capabilityCandidate
		for _, model := range models {
			capabilities, ok := maps[model.ID]
			if !ok {
				continue
			}
			assessment, ok := assessmentForPack(capabilities, definition.PackID)
			if !ok {
				continue
			}
			candidates = append(candidates, capabilityCandidate{model: model, assessment: assessment})
		}
		if len(candidates) == 0 {
			projection.Cells = append(projection.Cells, cell)
			continue
		}
		sortCapabilityCandidates(candidates)
		best := candidates[0]
		cell.State = best.assessment.State
		cell.Strength = best.assessment.Strength
		cell.Freshness = best.assessment.Freshness
		cell.ComparableRuns = best.assessment.ComparableRuns
		cell.SourceRunIDs = append([]string(nil), best.assessment.SourceRunIDs...)
		cell.RuleVersion = capabilityRuleVersion
		cell.PackVersion = best.assessment.PackVersion
		cell.Limitations = append(cell.Limitations, best.assessment.Notes...)
		if best.assessment.State == "SUPPORTED" || best.assessment.State == "STRONG" {
			cell.EvidenceLeader = &territoryLeader{ModelID: best.model.ID, ModelName: best.model.Name, State: best.assessment.State, Strength: best.assessment.Strength, Freshness: best.assessment.Freshness, ComparableRuns: best.assessment.ComparableRuns, PassRate: best.assessment.PassRate, Coverage: best.assessment.Coverage}
		}
		projection.Cells = append(projection.Cells, cell)
	}
	return projection, nil
}
