package main

import (
	"context"
	"fmt"
	"strings"
)

func (a *localApplication) ListMissions(ctx context.Context) ([]appMissionDefinition, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := append([]appMissionDefinition(nil), appMissions...)
	return result, nil
}

func findAppMission(reference string) (appMissionDefinition, error) {
	for _, mission := range appMissions {
		if strings.EqualFold(mission.ID, reference) {
			return mission, nil
		}
	}
	return appMissionDefinition{}, fmt.Errorf("mission %q not found", reference)
}

func requirementReason(assessment capabilityAssessment) string {
	switch {
	case assessment.ComparableRuns == 0:
		return "No comparable current evidence exists for this pack."
	case assessment.Freshness == "STALE":
		return "Existing evidence is stale and needs a bounded refresh."
	case assessment.Freshness == "AGING":
		return "Evidence is aging; refresh it before treating this prerequisite as current."
	case assessment.State == "RUNTIME_UNRELIABLE":
		return "Current evidence shows runtime unreliability; rerun to determine whether the condition persists."
	case assessment.State == "REVIEW_REQUIRED":
		return "The current evidence includes unresolved human-review requirements."
	case assessment.State == "WEAK" || assessment.State == "MIXED" || assessment.State == "PROMISING":
		return "Evidence exists but does not currently support this prerequisite."
	default:
		return "Additional current evidence is required."
	}
}

func (a *localApplication) PlanMission(ctx context.Context, missionID, modelRef, profileRef string) (missionPlan, error) {
	mission, err := findAppMission(missionID)
	if err != nil {
		return missionPlan{}, err
	}
	if profileRef == "" {
		profileRef = "default"
	}
	model, err := resolveModel(modelRef)
	if err != nil {
		return missionPlan{}, err
	}
	if err := ctx.Err(); err != nil {
		return missionPlan{}, err
	}
	capabilities, err := buildModelCapabilityMap(model.ID, profileRef, a.now())
	if err != nil {
		return missionPlan{}, err
	}
	plan := missionPlan{MissionID: mission.ID, Title: mission.Title, Question: mission.Question, ModelID: model.ID, ModelName: model.Name, ProfileID: profileRef}
	for _, packID := range mission.PackIDs {
		pack, packErr := findPack(packID)
		if packErr != nil {
			return missionPlan{}, packErr
		}
		assessment, ok := assessmentForPack(capabilities, packID)
		if !ok {
			assessment = capabilityAssessment{PackID: packID, PackTitle: pack.Title, PackVersion: pack.Version, State: "UNKNOWN", Strength: "NONE", Freshness: "UNKNOWN"}
		}
		satisfied := (assessment.State == "SUPPORTED" || assessment.State == "STRONG") && assessment.Freshness == "CURRENT"
		requirement := missionRequirement{ID: packID, Title: pack.Title, PackID: packID, State: assessment.State, Freshness: assessment.Freshness, Satisfied: satisfied, Runnable: !satisfied}
		if satisfied {
			requirement.Reason = "Current comparable evidence already satisfies this prerequisite; LocalCTL will not rerun it just to show progress."
		} else {
			requirement.Reason = requirementReason(assessment)
			plan.RunnablePacks = append(plan.RunnablePacks, packID)
		}
		plan.Requirements = append(plan.Requirements, requirement)
	}
	for _, requirementID := range mission.NonPackRequirements {
		reason := "No canonical, versioned LocalCTL experiment currently measures this requirement. It remains UNKNOWN."
		title := requirementID
		if requirementID == "agent-tool-use" {
			title = "Agent / tool use"
			reason = "Coding, structured-output, and reasoning evidence are prerequisites, but LocalCTL does not yet have a canonical tool-use pack. This mission cannot claim agent viability from proxy evidence."
		}
		plan.Requirements = append(plan.Requirements, missionRequirement{ID: requirementID, Title: title, State: "UNKNOWN", Freshness: "UNKNOWN", Satisfied: false, Runnable: false, Reason: reason})
		plan.Unresolved = append(plan.Unresolved, requirementID)
	}
	plan.Complete = len(plan.RunnablePacks) == 0 && len(plan.Unresolved) == 0
	return plan, nil
}

func (a *localApplication) GetMissionState(ctx context.Context, missionID, modelRef, profileRef string) (missionPlan, error) {
	return a.PlanMission(ctx, missionID, modelRef, profileRef)
}
