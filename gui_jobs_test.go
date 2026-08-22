package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func waitForJobState(t *testing.T, manager *jobManager, id string, terminal bool) jobRecord {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		record, err := manager.Get(id)
		if err == nil && terminalJobState(record.State) == terminal {
			return record
		}
		time.Sleep(10 * time.Millisecond)
	}
	record, _ := manager.Get(id)
	t.Fatalf("job %s did not reach expected terminal=%v; state=%s", id, terminal, record.State)
	return jobRecord{}
}

func newTestJobManager(t *testing.T) (*jobManager, modelArtifact) {
	t.Helper()
	home := useTempLocalCTLHome(t)
	model := seedTestModel(t, home, "job-Q4_K_M.gguf")
	manager, err := newJobManager(newLocalApplication())
	if err != nil {
		t.Fatal(err)
	}
	return manager, model
}

func TestJobSuccessfulLifecycleIsDurable(t *testing.T) {
	manager, model := newTestJobManager(t)
	manager.runner = func(ctx context.Context, record *jobRecord) {
		_ = manager.transition(record, jobStartingRuntime, "runtime.starting", nil)
		_ = manager.transition(record, jobRunning, "experiment.started", nil)
		_ = manager.transition(record, jobEvaluating, "experiment.completed", nil)
		_ = manager.transition(record, jobPersisting, "job.persisting", nil)
		_ = manager.transition(record, jobCompleted, "job.completed", nil)
	}
	record, created, err := manager.StartMission(context.Background(), "developer", model.ID, "default")
	if err != nil || !created {
		t.Fatalf("start: created=%v err=%v", created, err)
	}
	final := waitForJobState(t, manager, record.ID, true)
	if final.State != jobCompleted {
		t.Fatalf("state=%s", final.State)
	}
	persisted, err := loadJob(record.ID)
	if err != nil || persisted.State != jobCompleted {
		t.Fatalf("persisted=%#v err=%v", persisted, err)
	}
	want := map[string]bool{"job.planned": false, "job.queued": false, "runtime.starting": false, "experiment.started": false, "job.completed": false}
	for _, event := range persisted.Events {
		if _, ok := want[event.Type]; ok {
			want[event.Type] = true
		}
	}
	for event, seen := range want {
		if !seen {
			t.Fatalf("missing durable event %s", event)
		}
	}
}

func TestJobDuplicateSubmissionReturnsSameActiveJob(t *testing.T) {
	manager, model := newTestJobManager(t)
	started := make(chan struct{})
	manager.runner = func(ctx context.Context, record *jobRecord) {
		_ = manager.transition(record, jobRunning, "experiment.started", nil)
		close(started)
		<-ctx.Done()
		manager.cancelRecord(record, nil, nil, "test cancellation")
	}
	first, created, err := manager.StartMission(context.Background(), "developer", model.ID, "default")
	if err != nil || !created {
		t.Fatal(err)
	}
	<-started
	second, createdAgain, err := manager.StartMission(context.Background(), "developer", model.ID, "default")
	if err != nil {
		t.Fatal(err)
	}
	if createdAgain || second.ID != first.ID {
		t.Fatalf("duplicate created=%v first=%s second=%s", createdAgain, first.ID, second.ID)
	}
	_, _ = manager.Cancel(first.ID)
	final := waitForJobState(t, manager, first.ID, true)
	if final.State != jobCancelled {
		t.Fatalf("state=%s", final.State)
	}
}

func TestJobFailureClassesRemainDistinct(t *testing.T) {
	for _, test := range []struct {
		name  string
		class string
	}{
		{"runtime startup", "runtime_startup_failure"},
		{"inference", "inference_failure"},
		{"evaluation", "evaluation_failure"},
		{"persistence", "persistence_failure"},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager, model := newTestJobManager(t)
			manager.runner = func(ctx context.Context, record *jobRecord) {
				manager.fail(record, test.class, errors.New("fixture failure"))
			}
			record, _, err := manager.StartMission(context.Background(), "developer", model.ID, "default")
			if err != nil {
				t.Fatal(err)
			}
			final := waitForJobState(t, manager, record.ID, true)
			if final.State != jobFailed || final.ErrorClass != test.class {
				t.Fatalf("state=%s class=%s", final.State, final.ErrorClass)
			}
		})
	}
}

func TestJobCancellationIsTerminalAndInspectable(t *testing.T) {
	manager, model := newTestJobManager(t)
	started := make(chan struct{})
	manager.runner = func(ctx context.Context, record *jobRecord) {
		_ = manager.transition(record, jobRunning, "experiment.started", nil)
		close(started)
		<-ctx.Done()
		manager.cancelRecord(record, nil, nil, "operator cancelled")
	}
	record, _, err := manager.StartMission(context.Background(), "developer", model.ID, "default")
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if _, err := manager.Cancel(record.ID); err != nil {
		t.Fatal(err)
	}
	final := waitForJobState(t, manager, record.ID, true)
	if final.State != jobCancelled || !final.CancelRequested {
		t.Fatalf("final=%#v", final)
	}
	foundRequest := false
	for _, event := range final.Events {
		if event.Type == "job.cancel_requested" {
			foundRequest = true
		}
	}
	if !foundRequest {
		t.Fatal("cancel request event was lost")
	}
}

func TestNonterminalJobBecomesInterruptedAfterManagerRestart(t *testing.T) {
	useTempLocalCTLHome(t)
	now := time.Now()
	record := jobRecord{SchemaVersion: jobSchemaVersion, ID: newJobID(now), MissionID: "developer", State: jobRunning, CreatedAt: now, UpdatedAt: now}
	if err := saveJob(record); err != nil {
		t.Fatal(err)
	}
	manager, err := newJobManager(newLocalApplication())
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := manager.Get(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.State != jobInterrupted || recovered.ErrorClass != "process_interrupted" {
		t.Fatalf("recovered=%#v", recovered)
	}
}

func TestCancelledInferenceDoesNotCreateFailureEvidence(t *testing.T) {
	home := useTempLocalCTLHome(t)
	model := seedTestModel(t, home, "cancel-Q4_K_M.gguf")
	pack, err := findPack("structured-output")
	if err != nil {
		t.Fatal(err)
	}
	items := packExercises(pack)
	if len(items) == 0 {
		t.Fatal("structured-output pack has no exercises")
	}

	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, _, runErr := runObservedItemContextAt(ctx, server.URL, items[0], model, defaultProfile())
		done <- runErr
	}()
	<-started
	cancel()
	if runErr := <-done; !errors.Is(runErr, context.Canceled) {
		t.Fatalf("run error = %v, want context.Canceled", runErr)
	}

	records, err := listObservations()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("operator cancellation created %d evidence records; want 0", len(records))
	}
}
