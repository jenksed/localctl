package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

func (a *localApplication) GetScoutItems(ctx context.Context, profileRef string) ([]scoutItem, error) {
	territory, err := a.GetTerritory(ctx, profileRef)
	if err != nil {
		return nil, err
	}
	models, err := a.ListModels(ctx, profileRef)
	if err != nil {
		return nil, err
	}
	var items []scoutItem
	cellByID := map[string]territoryCell{}
	for _, cell := range territory.Cells {
		cellByID[cell.ID] = cell
		if cell.PackID == "" {
			items = append(items, scoutItem{ID: "gap-" + cell.ID, Priority: 35, Kind: "coverage-gap", Title: "Define evidence for " + cell.Title, Why: strings.Join(cell.Limitations, " "), PackID: cell.PackID, Runnable: false})
			continue
		}
		priority := requalificationPriority(cell.State, cell.Freshness)
		if priority == 0 {
			continue
		}
		kind := "investigate"
		title := "Investigate " + cell.Title
		if cell.Freshness == "STALE" || cell.Freshness == "AGING" {
			kind = "refresh"
			title = "Refresh " + cell.Title
		}
		items = append(items, scoutItem{ID: "territory-" + cell.ID, Priority: priority, Kind: kind, Title: title, Why: fmt.Sprintf("%s is %s with %s freshness under rule %s.", cell.Title, cell.State, cell.Freshness, capabilityRuleVersion), PackID: cell.PackID, Runnable: true})
	}
	for _, model := range models {
		if !model.Tested {
			items = append(items, scoutItem{ID: "untested-" + model.ID, Priority: 75, Kind: "untested-model", Title: "Characterize " + model.Name, Why: "This model is installed but has no saved LocalCTL evidence. A bounded mission would turn inventory into knowledge.", ModelID: model.ID, MissionID: "developer", Runnable: true})
		}
	}
	coding := cellByID["coding"]
	structured := cellByID["structured-output"]
	toolUse := cellByID["agent-tool-use"]
	if (coding.State == "SUPPORTED" || coding.State == "STRONG") && (structured.State == "SUPPORTED" || structured.State == "STRONG") && toolUse.State == "UNKNOWN" {
		modelID := ""
		if coding.EvidenceLeader != nil {
			modelID = coding.EvidenceLeader.ModelID
		}
		items = append(items, scoutItem{ID: "coding-agent-prereqs", Priority: 80, Kind: "mission", Title: "Resolve coding-agent prerequisites", Why: "Coding and structured-output evidence are supported, but tool-use is still UNKNOWN. The mission can tighten measurable prerequisites without pretending that proxy evidence proves agent viability.", MissionID: "coding-agent", ModelID: modelID, Runnable: modelID != "", Limitations: []string{"Tool-use remains unresolved until a canonical tool-use experiment exists."}})
	}
	explore, _ := loadExploreState()
	for _, selection := range explore.Selected {
		items = append(items, scoutItem{ID: "candidate-" + modelSlug(candidateKey(selection.Candidate)), Priority: 45, Kind: "selected-candidate", Title: "Revisit candidate " + selection.Candidate.Name, Why: "You selected this remote candidate for investigation. Selection is not installation, compatibility, testing, or demonstrated capability.", Runnable: false})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority > items[j].Priority
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}
