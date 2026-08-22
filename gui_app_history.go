package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func listSessions() ([]sessionRecord, error) {
	root, err := sessionRoot()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var records []sessionRecord
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(root, entry.Name()))
		if readErr != nil {
			return nil, readErr
		}
		var record sessionRecord
		if decodeErr := json.Unmarshal(data, &record); decodeErr != nil {
			return nil, decodeErr
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].StartedAt.After(records[j].StartedAt) })
	return records, nil
}

func (a *localApplication) ListRuns(ctx context.Context, limit int) ([]runObservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	records, err := listObservations()
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}
	return records, nil
}

func (a *localApplication) GetRun(ctx context.Context, runID string) (runDetailProjection, error) {
	if !safeLocalID(runID) {
		return runDetailProjection{}, fmt.Errorf("invalid run id")
	}
	if err := ctx.Err(); err != nil {
		return runDetailProjection{}, err
	}
	record, runDir, err := findObservation(runID)
	if err != nil {
		return runDetailProjection{}, err
	}
	detail := runDetailProjection{Observation: record}
	detail.Judgment, _ = loadJudgment(runDir)
	if data, readErr := os.ReadFile(filepath.Join(runDir, "prompt.txt")); readErr == nil {
		detail.Prompt = string(data)
	}
	if data, readErr := os.ReadFile(filepath.Join(runDir, "response.txt")); readErr == nil {
		detail.Response = string(data)
	}
	return detail, nil
}

func safeLocalID(value string) bool {
	if value == "" || len(value) > 200 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func (a *localApplication) History(ctx context.Context, limit int) (historyProjection, error) {
	if err := ctx.Err(); err != nil {
		return historyProjection{}, err
	}
	sessions, err := listSessions()
	if err != nil {
		return historyProjection{}, err
	}
	experiments, err := listExperiments()
	if err != nil {
		return historyProjection{}, err
	}
	runs, err := a.ListRuns(ctx, limit)
	if err != nil {
		return historyProjection{}, err
	}
	snapshots, err := listCapabilitySnapshots()
	if err != nil {
		return historyProjection{}, err
	}
	result := historyProjection{Sessions: sessions, Experiments: experiments, Runs: runs, Snapshots: snapshots}
	for _, run := range runs {
		_, runDir, findErr := findObservation(run.RunID)
		if findErr != nil {
			continue
		}
		if judgment, judgmentErr := loadJudgment(runDir); judgmentErr == nil && judgment != nil {
			result.Judgments = append(result.Judgments, *judgment)
		}
	}
	if limit > 0 {
		if len(result.Sessions) > limit {
			result.Sessions = result.Sessions[:limit]
		}
		if len(result.Experiments) > limit {
			result.Experiments = result.Experiments[:limit]
		}
		if len(result.Snapshots) > limit {
			result.Snapshots = result.Snapshots[:limit]
		}
	}
	return result, nil
}
