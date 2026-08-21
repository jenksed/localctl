package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type evaluationKind string

const (
	evaluationExact       evaluationKind = "exact"
	evaluationContainsAll evaluationKind = "contains_all"
	evaluationJSONExact   evaluationKind = "json_exact"
	evaluationManual      evaluationKind = "manual"
)

type evaluationSpec struct {
	Kind         evaluationKind `json:"kind"`
	Expected     string         `json:"expected,omitempty"`
	Contains     []string       `json:"contains,omitempty"`
	ExpectedJSON string         `json:"expected_json,omitempty"`
}

type exercise struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Category    string         `json:"category"`
	Difficulty  string         `json:"difficulty"`
	Description string         `json:"description"`
	Prompt      string         `json:"prompt"`
	Baseline    bool           `json:"baseline"`
	Evaluation  evaluationSpec `json:"evaluation"`
}

type evaluationResult struct {
	Mode        string `json:"mode"`
	Status      string `json:"status"`
	FailureKind string `json:"failure_kind,omitempty"`
	Detail      string `json:"detail,omitempty"`
}

func allExerciseCatalog() []exercise {
	items := append([]exercise{}, baselineExerciseCatalog()...)
	items = append(items, expandedExerciseCatalog()...)
	items = append(items, developerWorkflowExerciseCatalog()...)
	return items
}

func findExercise(id string) (exercise, bool) {
	for _, candidate := range allExerciseCatalog() {
		if candidate.ID == id {
			return candidate, true
		}
	}
	return exercise{}, false
}

func exerciseCategories() []string {
	seen := map[string]bool{}
	var categories []string
	for _, item := range allExerciseCatalog() {
		if !seen[item.Category] {
			seen[item.Category] = true
			categories = append(categories, item.Category)
		}
	}
	sort.Strings(categories)
	return categories
}

func exercisesForCategory(category string, includeExtended bool) []exercise {
	var result []exercise
	for _, item := range allExerciseCatalog() {
		if category != "" && item.Category != category {
			continue
		}
		if !includeExtended && !item.Baseline {
			continue
		}
		result = append(result, item)
	}
	return result
}

func evaluateExercise(item exercise, response string) evaluationResult {
	switch item.Evaluation.Kind {
	case evaluationExact:
		expected := strings.TrimSpace(item.Evaluation.Expected)
		actual := strings.TrimSpace(response)
		if actual == expected {
			return evaluationResult{Mode: string(evaluationExact), Status: "pass"}
		}
		failureKind := "incorrect_answer"
		if actual == "" {
			failureKind = "empty_output"
		} else if strings.HasPrefix(actual, expected) {
			failureKind = "contract_extra_output"
		}
		return evaluationResult{
			Mode:        string(evaluationExact),
			Status:      "fail",
			FailureKind: failureKind,
			Detail:      fmt.Sprintf("expected %q, got %q", expected, actual),
		}

	case evaluationContainsAll:
		actual := strings.ToLower(response)
		var missing []string
		for _, required := range item.Evaluation.Contains {
			if !strings.Contains(actual, strings.ToLower(required)) {
				missing = append(missing, required)
			}
		}
		if len(missing) == 0 {
			return evaluationResult{Mode: string(evaluationContainsAll), Status: "pass"}
		}
		failureKind := "missing_required_concepts"
		if strings.TrimSpace(response) == "" {
			failureKind = "empty_output"
		}
		return evaluationResult{
			Mode:        string(evaluationContainsAll),
			Status:      "fail",
			FailureKind: failureKind,
			Detail:      "missing required concepts: " + strings.Join(missing, ", "),
		}

	case evaluationJSONExact:
		var expected any
		if err := json.Unmarshal([]byte(item.Evaluation.ExpectedJSON), &expected); err != nil {
			return evaluationResult{
				Mode:        string(evaluationJSONExact),
				Status:      "error",
				FailureKind: "invalid_exercise",
				Detail:      "invalid exercise expectation: " + err.Error(),
			}
		}

		trimmed := strings.TrimSpace(response)
		if trimmed == "" {
			return evaluationResult{
				Mode:        string(evaluationJSONExact),
				Status:      "fail",
				FailureKind: "empty_output",
				Detail:      "response was empty",
			}
		}

		var actual any
		if err := json.Unmarshal([]byte(trimmed), &actual); err != nil {
			return evaluationResult{
				Mode:        string(evaluationJSONExact),
				Status:      "fail",
				FailureKind: "invalid_json",
				Detail:      "response was not valid JSON: " + err.Error(),
			}
		}

		if reflect.DeepEqual(actual, expected) {
			return evaluationResult{Mode: string(evaluationJSONExact), Status: "pass"}
		}

		return evaluationResult{
			Mode:        string(evaluationJSONExact),
			Status:      "fail",
			FailureKind: "structured_mismatch",
			Detail:      "valid JSON did not match the expected value",
		}

	case evaluationManual:
		return evaluationResult{
			Mode:   string(evaluationManual),
			Status: "pending",
			Detail: "requires human or later deterministic judgment",
		}

	default:
		return evaluationResult{
			Mode:        string(item.Evaluation.Kind),
			Status:      "error",
			FailureKind: "unknown_evaluator",
			Detail:      "unknown evaluation mode",
		}
	}
}
