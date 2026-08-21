package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const evidenceSchemaVersion = 2

type runObservation struct {
	SchemaVersion int       `json:"schema_version"`
	RunID         string    `json:"run_id"`
	StartedAt     time.Time `json:"started_at"`
	CompletedAt   time.Time `json:"completed_at"`
	Exercise      struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		Category     string `json:"category"`
		Difficulty   string `json:"difficulty"`
		PromptSHA256 string `json:"prompt_sha256,omitempty"`
	} `json:"exercise"`
	Model struct {
		ID                  string `json:"id"`
		Name                string `json:"name"`
		Path                string `json:"path"`
		Size                int64  `json:"size_bytes"`
		ArtifactMetadataKey string `json:"artifact_metadata_key,omitempty"`
	} `json:"model"`
	Runtime struct {
		Kind       string `json:"kind"`
		URL        string `json:"url"`
		PID        int    `json:"pid,omitempty"`
		Executable string `json:"executable,omitempty"`
		StartedAt  string `json:"started_at,omitempty"`
	} `json:"runtime"`
	Configuration struct {
		Context     int     `json:"context"`
		Temperature float64 `json:"temperature"`
		MaxTokens   int     `json:"max_tokens"`
	} `json:"configuration"`
	Result struct {
		Status              string  `json:"status"`
		FinishReason        string  `json:"finish_reason,omitempty"`
		PromptTokens        int     `json:"prompt_tokens,omitempty"`
		CompletionTokens    int     `json:"completion_tokens,omitempty"`
		TotalTokens         int     `json:"total_tokens,omitempty"`
		ElapsedMS           int64   `json:"elapsed_ms"`
		PromptTokensPerSec  float64 `json:"prompt_tokens_per_second,omitempty"`
		GenerationTokensSec float64 `json:"generation_tokens_per_second,omitempty"`
		ResponseSHA256      string  `json:"response_sha256,omitempty"`
		VisibleCharacters   int     `json:"visible_characters,omitempty"`
		EmptyVisibleOutput  bool    `json:"empty_visible_output,omitempty"`
		Error               string  `json:"error,omitempty"`
	} `json:"result"`
	Evaluation evaluationResult `json:"evaluation"`
}

type humanJudgment struct {
	RunID     string    `json:"run_id"`
	Verdict   string    `json:"verdict"`
	Reason    string    `json:"reason,omitempty"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

func newRunID(now time.Time) string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "run_" + now.UTC().Format("20060102T150405.000000000")
	}
	return "run_" + now.UTC().Format("20060102T150405.000") + "_" + hex.EncodeToString(buf)
}

func textSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func artifactMetadataKey(model modelArtifact) string {
	return textSHA256(fmt.Sprintf("%s\n%d", model.Path, model.Size))
}

func evidenceRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".localctl", "runs"), nil
}

func persistObservation(item exercise, model modelArtifact, startedAt time.Time, outcome inferenceOutcome, inferenceErr error) (runObservation, string, error) {
	completedAt := time.Now()
	record := runObservation{
		SchemaVersion: evidenceSchemaVersion,
		RunID:         newRunID(startedAt),
		StartedAt:     startedAt,
		CompletedAt:   completedAt,
	}

	record.Exercise.ID = item.ID
	record.Exercise.Title = item.Title
	record.Exercise.Category = item.Category
	record.Exercise.Difficulty = item.Difficulty
	record.Exercise.PromptSHA256 = textSHA256(item.Prompt)
	record.Model.ID = model.ID
	record.Model.Name = model.Name
	record.Model.Path = model.Path
	record.Model.Size = model.Size
	record.Model.ArtifactMetadataKey = artifactMetadataKey(model)
	record.Runtime.Kind = "llama.cpp"
	record.Runtime.URL = runtimeURL
	if state, err := readRuntimeState(); err == nil && state.Model == model.Path {
		record.Runtime.PID = state.PID
		record.Runtime.Executable = state.Executable
		record.Runtime.StartedAt = state.StartedAt.Format(time.RFC3339Nano)
	}
	record.Configuration.Context = 2048
	record.Configuration.Temperature = 0
	record.Configuration.MaxTokens = 512
	record.Result.ElapsedMS = outcome.Elapsed.Milliseconds()

	if inferenceErr != nil {
		record.Result.Status = "failed"
		record.Result.Error = inferenceErr.Error()
		record.Evaluation = evaluationResult{Mode: "not_evaluated", Status: "not_evaluated", FailureKind: "inference_error", Detail: "inference failed"}
	} else {
		record.Result.Status = "succeeded"
		record.Result.FinishReason = outcome.FinishReason
		record.Result.PromptTokens = outcome.PromptTokens
		record.Result.CompletionTokens = outcome.CompletionTokens
		record.Result.TotalTokens = outcome.TotalTokens
		record.Result.PromptTokensPerSec = outcome.PromptPerSecond
		record.Result.GenerationTokensSec = outcome.GenerationPerSecond
		record.Result.ResponseSHA256 = textSHA256(outcome.Content)
		record.Result.VisibleCharacters = len([]rune(outcome.Content))
		record.Result.EmptyVisibleOutput = strings.TrimSpace(outcome.Content) == ""
		record.Evaluation = evaluateExercise(item, outcome.Content)
	}

	root, err := evidenceRoot()
	if err != nil {
		return runObservation{}, "", err
	}

	runDir := filepath.Join(
		root,
		startedAt.Format("2006"),
		startedAt.Format("01"),
		startedAt.Format("02"),
		record.RunID,
	)
	if err := os.MkdirAll(runDir, 0700); err != nil {
		return runObservation{}, "", err
	}

	observationJSON, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return runObservation{}, "", err
	}

	if err := os.WriteFile(filepath.Join(runDir, "observation.json"), observationJSON, 0600); err != nil {
		return runObservation{}, "", err
	}
	if err := os.WriteFile(filepath.Join(runDir, "prompt.txt"), []byte(item.Prompt), 0600); err != nil {
		return runObservation{}, "", err
	}
	if inferenceErr == nil {
		if err := os.WriteFile(filepath.Join(runDir, "response.txt"), []byte(outcome.Content), 0600); err != nil {
			return runObservation{}, "", err
		}
	}

	if err := appendObservationIndex(root, record); err != nil {
		return runObservation{}, "", err
	}

	return record, runDir, nil
}

func appendObservationIndex(root string, record runObservation) error {
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}

	file, err := os.OpenFile(filepath.Join(root, "index.ndjson"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	line, err := json.Marshal(record)
	if err != nil {
		return err
	}

	_, err = file.Write(append(line, '\n'))
	return err
}

func listObservations() ([]runObservation, error) {
	root, err := evidenceRoot()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(filepath.Join(root, "index.ndjson"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var result []runObservation
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record runObservation
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, fmt.Errorf("decode evidence index: %w", err)
		}
		result = append(result, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})
	return result, nil
}

func scanObservationFiles() ([]runObservation, map[string]string, error) {
	root, err := evidenceRoot()
	if err != nil {
		return nil, nil, err
	}
	var records []runObservation
	paths := map[string]string{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if entry.IsDir() || entry.Name() != "observation.json" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		var record runObservation
		if decodeErr := json.Unmarshal(data, &record); decodeErr != nil {
			return fmt.Errorf("decode %s: %w", path, decodeErr)
		}
		records = append(records, record)
		paths[record.RunID] = filepath.Dir(path)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(records, func(i, j int) bool { return records[i].StartedAt.Before(records[j].StartedAt) })
	return records, paths, nil
}

func rebuildObservationIndex() (int, error) {
	root, err := evidenceRoot()
	if err != nil {
		return 0, err
	}
	records, _, err := scanObservationFiles()
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return 0, err
	}
	tmp := filepath.Join(root, "index.ndjson.tmp")
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return 0, err
	}
	for _, record := range records {
		line, marshalErr := json.Marshal(record)
		if marshalErr != nil {
			_ = file.Close()
			return 0, marshalErr
		}
		if _, writeErr := file.Write(append(line, '\n')); writeErr != nil {
			_ = file.Close()
			return 0, writeErr
		}
	}
	if err := file.Close(); err != nil {
		return 0, err
	}
	if err := os.Rename(tmp, filepath.Join(root, "index.ndjson")); err != nil {
		return 0, err
	}
	return len(records), nil
}

func findObservation(runID string) (runObservation, string, error) {
	root, err := evidenceRoot()
	if err != nil {
		return runObservation{}, "", err
	}

	var foundPath string
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if entry.IsDir() || entry.Name() != "observation.json" {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) == runID || strings.HasPrefix(filepath.Base(filepath.Dir(path)), runID) {
			foundPath = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return runObservation{}, "", err
	}

	if foundPath == "" {
		return runObservation{}, "", fmt.Errorf("run %q not found", runID)
	}

	data, err := os.ReadFile(foundPath)
	if err != nil {
		return runObservation{}, "", err
	}

	var record runObservation
	if err := json.Unmarshal(data, &record); err != nil {
		return runObservation{}, "", err
	}
	return record, filepath.Dir(foundPath), nil
}

func saveJudgment(runID, verdict, reason string) (humanJudgment, error) {
	_, runDir, err := findObservation(runID)
	if err != nil {
		return humanJudgment{}, err
	}

	judgment := humanJudgment{
		RunID:     filepath.Base(runDir),
		Verdict:   verdict,
		Reason:    reason,
		Source:    "human",
		CreatedAt: time.Now(),
	}

	data, err := json.MarshalIndent(judgment, "", "  ")
	if err != nil {
		return humanJudgment{}, err
	}
	if err := os.WriteFile(filepath.Join(runDir, "judgment.json"), data, 0600); err != nil {
		return humanJudgment{}, err
	}
	return judgment, nil
}

func loadJudgment(runDir string) (*humanJudgment, error) {
	data, err := os.ReadFile(filepath.Join(runDir, "judgment.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var judgment humanJudgment
	if err := json.Unmarshal(data, &judgment); err != nil {
		return nil, err
	}
	return &judgment, nil
}
