package main

import "testing"

func TestExerciseCatalog(t *testing.T) {
	catalog := allExerciseCatalog()
	if len(catalog) < 180 {
		t.Fatalf("expected a broad real-world exercise catalog, got %d", len(catalog))
	}

	seen := map[string]bool{}
	baselineCount := 0
	manualCount := 0
	categoryCounts := map[string]int{}

	for _, item := range catalog {
		if item.ID == "" || item.Title == "" || item.Category == "" || item.Prompt == "" {
			t.Fatalf("exercise has missing required fields: %#v", item)
		}
		if seen[item.ID] {
			t.Fatalf("duplicate exercise id: %s", item.ID)
		}
		seen[item.ID] = true
		categoryCounts[item.Category]++
		if item.Baseline {
			baselineCount++
		}
		if item.Evaluation.Kind == evaluationManual {
			manualCount++
		}
	}

	if baselineCount != 18 {
		t.Fatalf("expected the fast core baseline to remain 18 exercises, got %d", baselineCount)
	}
	if manualCount < 15 {
		t.Fatalf("expected substantial human-judged real-work exercises, got %d", manualCount)
	}
	if categoryCounts["linux"] != 34 {
		t.Fatalf("expected 34 Linux investigation scenarios, got %d", categoryCounts["linux"])
	}
	if categoryCounts["docker"] != 33 {
		t.Fatalf("expected 33 Docker investigation scenarios, got %d", categoryCounts["docker"])
	}
	if categoryCounts["kubernetes"] != 33 {
		t.Fatalf("expected 33 Kubernetes investigation scenarios, got %d", categoryCounts["kubernetes"])
	}
	if categoryCounts["linux"]+categoryCounts["docker"]+categoryCounts["kubernetes"] != 100 {
		t.Fatalf("expected exactly 100 infrastructure investigation scenarios")
	}
	for _, category := range []string{"developer", "summarization", "security", "data", "context", "operations", "writing", "planning"} {
		if categoryCounts[category] == 0 {
			t.Fatalf("expected category %q to contain scenarios", category)
		}
	}
}

func TestEvaluateExercise(t *testing.T) {
	tests := []struct {
		name            string
		item            exercise
		response        string
		wantStatus      string
		wantFailureKind string
	}{
		{
			name: "exact pass",
			item: exercise{Evaluation: evaluationSpec{
				Kind:     evaluationExact,
				Expected: "LOCALCTL_OK",
			}},
			response:   "LOCALCTL_OK\n",
			wantStatus: "pass",
		},
		{
			name: "exact incorrect",
			item: exercise{Evaluation: evaluationSpec{
				Kind:     evaluationExact,
				Expected: "LOCALCTL_OK",
			}},
			response:        "WRONG",
			wantStatus:      "fail",
			wantFailureKind: "incorrect_answer",
		},
		{
			name: "exact answer plus explanation is contract failure",
			item: exercise{Evaluation: evaluationSpec{
				Kind:     evaluationExact,
				Expected: "NO",
			}},
			response:        "NO\n\nExplanation: additional text",
			wantStatus:      "fail",
			wantFailureKind: "contract_extra_output",
		},
		{
			name: "exact empty output",
			item: exercise{Evaluation: evaluationSpec{
				Kind:     evaluationExact,
				Expected: "NO",
			}},
			response:        "",
			wantStatus:      "fail",
			wantFailureKind: "empty_output",
		},
		{
			name: "contains pass",
			item: exercise{Evaluation: evaluationSpec{
				Kind:     evaluationContainsAll,
				Contains: []string{"value", "not used"},
			}},
			response:   "The variable VALUE is declared but NOT USED.",
			wantStatus: "pass",
		},
		{
			name: "json pass independent of key order",
			item: exercise{Evaluation: evaluationSpec{
				Kind:         evaluationJSONExact,
				ExpectedJSON: `{"status":"ok","count":3}`,
			}},
			response:   `{"count":3,"status":"ok"}`,
			wantStatus: "pass",
		},
		{
			name: "invalid json classified",
			item: exercise{Evaluation: evaluationSpec{
				Kind:         evaluationJSONExact,
				ExpectedJSON: `{"status":"ok"}`,
			}},
			response:        "status=ok",
			wantStatus:      "fail",
			wantFailureKind: "invalid_json",
		},
		{
			name:       "manual pending",
			item:       exercise{Evaluation: evaluationSpec{Kind: evaluationManual}},
			response:   "plausible answer",
			wantStatus: "pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateExercise(tt.item, tt.response)
			if result.Status != tt.wantStatus {
				t.Fatalf("expected status %s, got %#v", tt.wantStatus, result)
			}
			if result.FailureKind != tt.wantFailureKind {
				t.Fatalf("expected failure kind %q, got %#v", tt.wantFailureKind, result)
			}
		})
	}
}
