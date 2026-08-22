package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const jobSchemaVersion = 1

const (
	jobPlanned         = "PLANNED"
	jobQueued          = "QUEUED"
	jobStartingRuntime = "STARTING_RUNTIME"
	jobRunning         = "RUNNING"
	jobEvaluating      = "EVALUATING"
	jobPersisting      = "PERSISTING"
	jobCompleted       = "COMPLETED"
	jobFailed          = "FAILED"
	jobCancelled       = "CANCELLED"
	jobInterrupted     = "INTERRUPTED"
)

type jobEvent struct {
	Sequence int                    `json:"sequence"`
	Type     string                 `json:"type"`
	At       time.Time              `json:"at"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

type jobRecord struct {
	SchemaVersion        int         `json:"schema_version"`
	ID                   string      `json:"id"`
	MissionID            string      `json:"mission_id"`
	MissionTitle         string      `json:"mission_title"`
	ModelID              string      `json:"model_id"`
	ModelName            string      `json:"model_name"`
	ProfileID            string      `json:"profile_id"`
	Plan                 missionPlan `json:"plan"`
	State                string      `json:"state"`
	ErrorClass           string      `json:"error_class,omitempty"`
	Error                string      `json:"error,omitempty"`
	CancelRequested      bool        `json:"cancel_requested,omitempty"`
	CreatedAt            time.Time   `json:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at"`
	CompletedAt          *time.Time  `json:"completed_at,omitempty"`
	SessionID            string      `json:"session_id,omitempty"`
	ExperimentIDs        []string    `json:"experiment_ids,omitempty"`
	SourceRunIDs         []string    `json:"source_run_ids,omitempty"`
	CapabilitySnapshotID string      `json:"capability_snapshot_id,omitempty"`
	Events               []jobEvent  `json:"events,omitempty"`
}

type jobManager struct {
	app      *localApplication
	mu       sync.Mutex
	activeID string
	cancels  map[string]context.CancelFunc
	runner   func(context.Context, *jobRecord)
}

func newJobManager(app *localApplication) (*jobManager, error) {
	manager := &jobManager{app: app, cancels: map[string]context.CancelFunc{}}
	manager.runner = manager.runMissionJob
	if err := manager.recoverInterruptedJobs(); err != nil {
		return nil, err
	}
	return manager, nil
}

func newJobID(now time.Time) string {
	return strings.Replace(newRunID(now), "run_", "job_", 1)
}

func terminalJobState(state string) bool {
	switch state {
	case jobCompleted, jobFailed, jobCancelled, jobInterrupted:
		return true
	default:
		return false
	}
}

func jobPath(id string) (string, error) {
	if !safeLocalID(id) {
		return "", fmt.Errorf("invalid job id")
	}
	root, err := jobsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, id+".json"), nil
}

func saveJob(record jobRecord) error {
	path, err := jobPath(record.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadJob(id string) (jobRecord, error) {
	path, err := jobPath(id)
	if err != nil {
		return jobRecord{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return jobRecord{}, err
	}
	var record jobRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return jobRecord{}, err
	}
	return record, nil
}

func listJobs() ([]jobRecord, error) {
	root, err := jobsRoot()
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
	var records []jobRecord
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(root, entry.Name()))
		if readErr != nil {
			return nil, readErr
		}
		var record jobRecord
		if decodeErr := json.Unmarshal(data, &record); decodeErr != nil {
			return nil, decodeErr
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].CreatedAt.After(records[j].CreatedAt) })
	return records, nil
}

func (m *jobManager) recoverInterruptedJobs() error {
	records, err := listJobs()
	if err != nil {
		return err
	}
	now := m.app.now()
	for _, record := range records {
		if terminalJobState(record.State) {
			continue
		}
		record.State = jobInterrupted
		record.ErrorClass = "process_interrupted"
		record.Error = "LocalCTL restarted while this job was not in a terminal state. The job was not silently resumed."
		record.UpdatedAt = now
		record.CompletedAt = &now
		appendJobEvent(&record, "job.interrupted", map[string]interface{}{"reason": record.Error})
		if err := saveJob(record); err != nil {
			return err
		}
	}
	return nil
}

func appendJobEvent(record *jobRecord, eventType string, data map[string]interface{}) {
	now := time.Now()
	event := jobEvent{Sequence: len(record.Events) + 1, Type: eventType, At: now, Data: data}
	record.Events = append(record.Events, event)
	// Mission plans are bounded by the fixed experiment-pack catalog, so retaining
	// the complete event history keeps SSE cursors monotonic without creating an
	// unbounded producer. Durable job state remains the authority.
	record.UpdatedAt = now
}

func mergeLatestJobState(record *jobRecord) {
	latest, err := loadJob(record.ID)
	if err != nil {
		return
	}
	if len(latest.Events) > len(record.Events) {
		record.Events = append([]jobEvent(nil), latest.Events...)
	}
	if latest.CancelRequested {
		record.CancelRequested = true
	}
}

func (m *jobManager) saveRecordLocked(record *jobRecord) error {
	mergeLatestJobState(record)
	return saveJob(*record)
}

func (m *jobManager) transition(record *jobRecord, state, eventType string, data map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mergeLatestJobState(record)
	record.State = state
	appendJobEvent(record, eventType, data)
	if terminalJobState(state) {
		now := record.UpdatedAt
		record.CompletedAt = &now
		if m.activeID == record.ID {
			m.activeID = ""
		}
		delete(m.cancels, record.ID)
	}
	return saveJob(*record)
}

func (m *jobManager) fail(record *jobRecord, class string, err error) {
	record.ErrorClass = class
	if err != nil {
		record.Error = err.Error()
	}
	_ = m.transition(record, jobFailed, "job.failed", map[string]interface{}{"class": class, "error": record.Error})
}

func (m *jobManager) StartMission(ctx context.Context, missionID, modelRef, profileRef string) (jobRecord, bool, error) {
	plan, err := m.app.PlanMission(ctx, missionID, modelRef, profileRef)
	if err != nil {
		return jobRecord{}, false, err
	}
	if len(plan.RunnablePacks) == 0 {
		if plan.Complete {
			return jobRecord{}, false, fmt.Errorf("mission already satisfied by current evidence")
		}
		return jobRecord{}, false, fmt.Errorf("mission has no runnable unresolved requirements in this release: %s", strings.Join(plan.Unresolved, ", "))
	}

	m.mu.Lock()
	if m.activeID != "" {
		active, loadErr := loadJob(m.activeID)
		if loadErr == nil && !terminalJobState(active.State) {
			if active.MissionID == plan.MissionID && active.ModelID == plan.ModelID && active.ProfileID == plan.ProfileID {
				m.mu.Unlock()
				return active, false, nil
			}
			m.mu.Unlock()
			return jobRecord{}, false, fmt.Errorf("job %s is active; LocalCTL serializes GUI experiments to protect shared runtime and observation scope", active.ID)
		}
		m.activeID = ""
	}
	now := m.app.now()
	record := jobRecord{SchemaVersion: jobSchemaVersion, ID: newJobID(now), MissionID: plan.MissionID, MissionTitle: plan.Title, ModelID: plan.ModelID, ModelName: plan.ModelName, ProfileID: plan.ProfileID, Plan: plan, State: jobPlanned, CreatedAt: now, UpdatedAt: now}
	appendJobEvent(&record, "job.planned", map[string]interface{}{"packs": append([]string(nil), plan.RunnablePacks...)})
	if err := saveJob(record); err != nil {
		m.mu.Unlock()
		return jobRecord{}, false, err
	}
	record.State = jobQueued
	appendJobEvent(&record, "job.queued", nil)
	if err := saveJob(record); err != nil {
		m.mu.Unlock()
		return jobRecord{}, false, err
	}
	jobCtx, cancel := context.WithCancel(context.Background())
	m.activeID = record.ID
	m.cancels[record.ID] = cancel
	m.mu.Unlock()

	go m.runner(jobCtx, &record)
	return record, true, nil
}

func (m *jobManager) Get(id string) (jobRecord, error) {
	return loadJob(id)
}

func (m *jobManager) List(limit int) ([]jobRecord, error) {
	records, err := listJobs()
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}
	return records, nil
}

func (m *jobManager) Cancel(id string) (jobRecord, error) {
	m.mu.Lock()
	record, err := loadJob(id)
	if err != nil {
		m.mu.Unlock()
		return jobRecord{}, err
	}
	if terminalJobState(record.State) {
		m.mu.Unlock()
		return record, nil
	}
	record.CancelRequested = true
	appendJobEvent(&record, "job.cancel_requested", nil)
	if err := saveJob(record); err != nil {
		m.mu.Unlock()
		return jobRecord{}, err
	}
	cancel := m.cancels[id]
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return record, nil
}

func (m *jobManager) cancelRecord(record *jobRecord, session *sessionRecord, experiment *experimentRecord, reason string) {
	if experiment != nil {
		now := m.app.now()
		experiment.Status = "cancelled"
		experiment.CompletedAt = now
		_ = saveExperiment(*experiment)
	}
	if session != nil {
		now := m.app.now()
		session.Status = "cancelled"
		session.CompletedAt = now
		_ = saveSession(*session)
	}
	record.CancelRequested = true
	_ = m.transition(record, jobCancelled, "job.cancelled", map[string]interface{}{"reason": reason})
}

func (m *jobManager) runMissionJob(ctx context.Context, record *jobRecord) {
	previousGlobalScope := scopedObservation
	defer func() { scopedObservation = previousGlobalScope }()
	model, err := resolveModel(record.ModelID)
	if err != nil {
		m.fail(record, "model_resolution_failure", err)
		return
	}
	profile, err := resolveProfile(record.ProfileID)
	if err != nil {
		m.fail(record, "profile_resolution_failure", err)
		return
	}

	session := sessionRecord{SchemaVersion: 1, ID: newSessionID(m.app.now()), Name: "gui mission " + record.MissionID + " " + model.ID, Kind: "gui-mission", StartedAt: m.app.now(), Status: "running"}
	if err := saveSession(session); err != nil {
		m.fail(record, "persistence_failure", err)
		return
	}
	record.SessionID = session.ID
	m.mu.Lock()
	_ = m.saveRecordLocked(record)
	m.mu.Unlock()

	previousSessionID := activeSessionID
	activeSessionID = session.ID
	defer func() { activeSessionID = previousSessionID }()

	var currentExperiment *experimentRecord
	defer func() {
		if recovered := recover(); recovered != nil {
			if currentExperiment != nil {
				now := m.app.now()
				currentExperiment.Status = "failed"
				currentExperiment.CompletedAt = now
				_ = saveExperiment(*currentExperiment)
			}
			now := m.app.now()
			session.Status = "failed"
			session.CompletedAt = now
			_ = saveSession(session)
			m.fail(record, "internal_panic", fmt.Errorf("job panic: %v", recovered))
		}
	}()

	for _, packID := range record.Plan.RunnablePacks {
		if ctx.Err() != nil {
			m.cancelRecord(record, &session, currentExperiment, "cancelled before runtime start")
			return
		}
		pack, packErr := findPack(packID)
		if packErr != nil {
			m.fail(record, "mission_plan_failure", packErr)
			return
		}
		if err := m.transition(record, jobStartingRuntime, "runtime.starting", map[string]interface{}{"model_id": model.ID, "profile_id": profile.ID, "pack_id": pack.ID}); err != nil {
			m.fail(record, "persistence_failure", err)
			return
		}
		var runtimeOut, runtimeErr bytes.Buffer
		if _, code := ensureRuntimeForModelProfile(model, profile, &runtimeOut, &runtimeErr); code != 0 {
			m.fail(record, "runtime_startup_failure", errors.New(strings.TrimSpace(runtimeErr.String()+" "+runtimeOut.String())))
			return
		}
		if ctx.Err() != nil {
			m.cancelRecord(record, &session, nil, "cancelled while runtime startup was in progress; shared runtime was left in its reconciled state")
			return
		}
		appendData := map[string]interface{}{"model_id": model.ID, "profile_id": profile.ID}
		m.mu.Lock()
		appendJobEvent(record, "runtime.ready", appendData)
		_ = m.saveRecordLocked(record)
		m.mu.Unlock()

		now := m.app.now()
		experiment := experimentRecord{SchemaVersion: 1, ID: newExperimentID(now), SessionID: session.ID, Name: pack.ID + "/" + pack.Version, Kind: "gui-pack", ModelID: model.ID, ModelPath: model.Path, ProfileID: profile.ID, PackID: pack.ID, PackVersion: pack.Version, InputClass: "canonical", StartedAt: now, Status: "running"}
		if err := saveExperiment(experiment); err != nil {
			m.fail(record, "persistence_failure", err)
			return
		}
		currentExperiment = &experiment
		record.ExperimentIDs = append(record.ExperimentIDs, experiment.ID)
		m.mu.Lock()
		_ = m.saveRecordLocked(record)
		m.mu.Unlock()

		previousScope := scopedObservation
		scopedObservation = &observationScope{ExperimentID: experiment.ID, SessionID: session.ID, Experiment: experiment.Name, ExperimentKind: experiment.Kind, Profile: profile, Pack: pack, InputClass: "canonical"}
		if err := m.transition(record, jobRunning, "experiment.started", map[string]interface{}{"experiment_id": experiment.ID, "pack_id": pack.ID, "exercise_count": len(packExercises(pack))}); err != nil {
			scopedObservation = previousScope
			m.fail(record, "persistence_failure", err)
			return
		}

		for index, item := range packExercises(pack) {
			if ctx.Err() != nil {
				scopedObservation = previousScope
				m.cancelRecord(record, &session, &experiment, "cancelled between exercises")
				return
			}
			m.mu.Lock()
			appendJobEvent(record, "exercise.started", map[string]interface{}{"exercise_id": item.ID, "index": index + 1, "total": len(packExercises(pack)), "pack_id": pack.ID})
			_ = m.saveRecordLocked(record)
			m.mu.Unlock()
			observation, outcome, runErr := runObservedItemContext(ctx, item, model, profile)
			if ctx.Err() != nil {
				scopedObservation = previousScope
				m.cancelRecord(record, &session, &experiment, "cancelled during inference; cancellation was not recorded as model failure evidence")
				return
			}
			if runErr != nil {
				scopedObservation = previousScope
				now := m.app.now()
				experiment.Status = "failed"
				experiment.CompletedAt = now
				_ = saveExperiment(experiment)
				session.Status = "failed"
				session.CompletedAt = now
				_ = saveSession(session)
				if strings.Contains(runErr.Error(), "save evidence") {
					m.fail(record, "persistence_failure", runErr)
				} else {
					m.fail(record, "inference_failure", runErr)
				}
				return
			}
			record.SourceRunIDs = append(record.SourceRunIDs, observation.RunID)
			m.mu.Lock()
			appendJobEvent(record, "measurement.recorded", map[string]interface{}{"run_id": observation.RunID, "elapsed_ms": outcome.Elapsed.Milliseconds(), "generation_tokens_per_second": outcome.GenerationPerSecond})
			appendJobEvent(record, "observation.persisted", map[string]interface{}{"run_id": observation.RunID})
			appendJobEvent(record, "exercise.completed", map[string]interface{}{"exercise_id": item.ID, "run_id": observation.RunID, "evaluation": observation.Evaluation.Status, "failure_kind": observation.Evaluation.FailureKind})
			_ = m.saveRecordLocked(record)
			m.mu.Unlock()
		}
		scopedObservation = previousScope
		if err := m.transition(record, jobEvaluating, "experiment.completed", map[string]interface{}{"experiment_id": experiment.ID, "pack_id": pack.ID}); err != nil {
			m.fail(record, "persistence_failure", err)
			return
		}
		now = m.app.now()
		experiment.Status = "completed"
		experiment.CompletedAt = now
		if err := saveExperiment(experiment); err != nil {
			m.fail(record, "persistence_failure", err)
			return
		}
		currentExperiment = nil
	}

	if ctx.Err() != nil {
		m.cancelRecord(record, &session, nil, "cancelled before capability persistence")
		return
	}
	if err := m.transition(record, jobPersisting, "job.persisting", nil); err != nil {
		m.fail(record, "persistence_failure", err)
		return
	}
	capabilities, err := buildModelCapabilityMap(model.ID, profile.ID, m.app.now())
	if err != nil {
		m.fail(record, "evaluation_failure", err)
		return
	}
	snapshotID, _, err := persistCapabilityMap(capabilities)
	if err != nil {
		m.fail(record, "persistence_failure", err)
		return
	}
	record.CapabilitySnapshotID = snapshotID
	m.mu.Lock()
	appendJobEvent(record, "capability.updated", map[string]interface{}{"snapshot_id": snapshotID, "rule_version": capabilities.RuleVersion})
	_ = m.saveRecordLocked(record)
	m.mu.Unlock()

	now := m.app.now()
	session.Status = "completed"
	session.CompletedAt = now
	if err := saveSession(session); err != nil {
		m.fail(record, "persistence_failure", err)
		return
	}
	_ = m.transition(record, jobCompleted, "job.completed", map[string]interface{}{"snapshot_id": snapshotID})
}
