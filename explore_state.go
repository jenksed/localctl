package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type exploreSelection struct {
	Candidate  modelRadarCandidate `json:"candidate"`
	SelectedAt time.Time           `json:"selected_at"`
}

type exploreState struct {
	SchemaVersion int                `json:"schema_version"`
	Selected      []exploreSelection `json:"selected"`
}

func exploreStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".localctl", "explore.json"), nil
}

func loadExploreState() (exploreState, error) {
	path, err := exploreStatePath()
	if err != nil {
		return exploreState{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return exploreState{SchemaVersion: 1}, nil
		}
		return exploreState{}, err
	}
	var state exploreState
	if err := json.Unmarshal(data, &state); err != nil {
		return exploreState{}, fmt.Errorf("decode explore state: %w", err)
	}
	if state.SchemaVersion == 0 {
		state.SchemaVersion = 1
	}
	return state, nil
}

func saveExploreState(state exploreState) error {
	path, err := exploreStatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	state.SchemaVersion = 1
	sort.Slice(state.Selected, func(i, j int) bool {
		return state.Selected[i].SelectedAt.Before(state.Selected[j].SelectedAt)
	})
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func selectedCandidateIDs(state exploreState) map[string]bool {
	result := map[string]bool{}
	for _, item := range state.Selected {
		result[candidateKey(item.Candidate)] = true
	}
	return result
}

func candidateKey(candidate modelRadarCandidate) string {
	if strings.TrimSpace(candidate.Repository) != "" {
		return strings.ToLower(strings.TrimSpace(candidate.Repository))
	}
	return modelSlug(candidate.Name)
}

func candidateInstalled(candidate modelRadarCandidate, models []modelArtifact) bool {
	needles := candidateFamilyNeedles(candidate)
	for _, model := range models {
		for _, needle := range needles {
			if len(needle) < 4 {
				continue
			}
			if strings.Contains(model.ID, needle) || strings.Contains(needle, model.ID) {
				return true
			}
		}
	}
	return false
}

func candidateFamilyNeedles(candidate modelRadarCandidate) []string {
	seen := map[string]bool{}
	var result []string
	add := func(value string) {
		value = modelSlug(value)
		value = strings.TrimSuffix(value, "-gguf")
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		result = append(result, value)
	}
	add(candidate.Name)
	if candidate.Repository != "" {
		parts := strings.Split(strings.Trim(candidate.Repository, "/"), "/")
		add(parts[len(parts)-1])
	}
	return result
}

func pruneInstalledSelections(state exploreState, models []modelArtifact) (exploreState, bool) {
	kept := state.Selected[:0]
	changed := false
	for _, item := range state.Selected {
		if candidateInstalled(item.Candidate, models) {
			changed = true
			continue
		}
		kept = append(kept, item)
	}
	state.Selected = kept
	return state, changed
}

func selectExploreCandidate(state exploreState, candidate modelRadarCandidate) exploreState {
	key := candidateKey(candidate)
	for _, item := range state.Selected {
		if candidateKey(item.Candidate) == key {
			return state
		}
	}
	state.Selected = append(state.Selected, exploreSelection{Candidate: candidate, SelectedAt: time.Now()})
	return state
}

func unselectExploreCandidate(state exploreState, reference string) (exploreState, bool) {
	needle := strings.ToLower(strings.TrimSpace(reference))
	kept := state.Selected[:0]
	removed := false
	for _, item := range state.Selected {
		candidate := item.Candidate
		haystack := strings.ToLower(candidateKey(candidate) + " " + candidate.Name + " " + candidate.Repository)
		if !removed && (candidateKey(candidate) == needle || strings.Contains(haystack, needle)) {
			removed = true
			continue
		}
		kept = append(kept, item)
	}
	state.Selected = kept
	return state, removed
}
