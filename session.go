package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type sessionRecord struct {
	SchemaVersion int       `json:"schema_version"`
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Kind          string    `json:"kind"`
	StartedAt     time.Time `json:"started_at"`
	CompletedAt   time.Time `json:"completed_at,omitempty"`
	Status        string    `json:"status"`
}

var activeSessionID string

func sessionRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".localctl", "sessions"), nil
}

func newSessionID(now time.Time) string {
	return strings.Replace(newRunID(now), "run_", "session_", 1)
}

func saveSession(record sessionRecord) error {
	root, err := sessionRoot()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, record.ID+".json"), data, 0600)
}

func startSession(kind, name string) (sessionRecord, func()) {
	now := time.Now()
	record := sessionRecord{SchemaVersion: 1, ID: newSessionID(now), Name: name, Kind: kind, StartedAt: now, Status: "running"}
	_ = saveSession(record)
	previous := activeSessionID
	activeSessionID = record.ID
	finish := func() {
		record.CompletedAt = time.Now()
		record.Status = "completed"
		_ = saveSession(record)
		activeSessionID = previous
	}
	return record, finish
}

func sessionSummary(record sessionRecord) string {
	return fmt.Sprintf("%s (%s)", record.ID, record.Name)
}
