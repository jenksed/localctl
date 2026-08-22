package main

import (
	"context"
	"os"
	"path/filepath"
)

func jobsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".localctl", "jobs"), nil
}

func (a *localApplication) Settings(ctx context.Context) (settingsProjection, error) {
	if err := ctx.Err(); err != nil {
		return settingsProjection{}, err
	}
	evidence, err := evidenceRoot()
	if err != nil {
		return settingsProjection{}, err
	}
	intelligence, err := intelligenceRoot()
	if err != nil {
		return settingsProjection{}, err
	}
	jobs, err := jobsRoot()
	if err != nil {
		return settingsProjection{}, err
	}
	profiles, err := listProfiles()
	if err != nil {
		return settingsProjection{}, err
	}
	return settingsProjection{BindAddress: "127.0.0.1", RuntimeURL: runtimeURL, EvidenceRoot: evidence, IntelligenceRoot: intelligence, JobsRoot: jobs, Profiles: profiles, Security: []string{"loopback-only binding", "Host validation", "same-origin mutation validation", "CSRF header on mutations", "no CORS", "no arbitrary command HTTP endpoint", "bounded request bodies"}, ReadIndex: filepath.Join(evidence, "index.ndjson"), ReadIndexAuthority: "rebuildable projection; observation.json files remain canonical evidence"}, nil
}

func (a *localApplication) RebuildReadIndex(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return rebuildObservationIndex()
}
