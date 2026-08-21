package main

import "testing"

func TestExerciseCatalog(t *testing.T) {
	catalog := baselineExerciseCatalog()
	if len(catalog) < 25 {
		t.Fatalf("expected a substantial exercise catalog, got %d", len(catalog))
	}

	seen := map[string]bool{}
	baselineCount := 0
	manualCount := 0

	for _, item := range catalog {
		if item.ID == "" || item.Title == "" || item.Category == "" || item.Prompt == "" {
			t.Fatalf("exercise has missing required fields: %#v", item)
		}
		if seen[item.ID] {
			t.Fatalf("duplicate exercise id: %s", item.ID)
		}
		seen[item.ID] = true
		if item.Baseline {
			baselineCount++
		}
		if item.Evaluation.Kind == evaluationManual {
			manualCount++
		}
	}

	if baselineCount < 12 {
		t.Fatalf("expected at least 12 core baseline exercises, got %d", baselineCount)
	}
	if manualCount == 0 {
		t.Fatalf("expected human-judged exercises as well as deterministic ones")
	}
}

func TestEvaluateExercise(t *testing.T) {
	tests := []struct {
		name     string
		item     exercise
		response string
		want     string
	}{
		{
			name: "exact pass",
			item: exercise{Evaluation: evaluationSpec{
				Kind:     evaluationExact,
				Expected: "LOCALCTL_OK",
			}},
			response: "LOCALCTL_OK\n",
			want:     "pass",
		},
		{
			name: "exact fail",
			item: exercise{Evaluation: evaluationSpec{
				Kind:     evaluationExact,
				Expected: "LOCALCTL_OK",
			}},
			response: "Sure: LOCALCTL_OK",
			want:     "fail",
		},
		{
			name: "contains pass",
			item: exercise{Evaluation: evaluationSpec{
				Kind:     evaluationContainsAll,
				Contains: []string{"value", "not used"},
			}},
			response: "The variable VALUE is declared but NOT USED.",
			want:     "pass",
		},
		{
			name: "json pass independent of key order",
			item: exercise{Evaluation: evaluationSpec{
				Kind:         evaluationJSONExact,
				ExpectedJSON: `{"status":"ok","count":3}`,
			}},
			response: `{"count":3,"status":"ok"}`,
			want:     "pass",
		},
		{
			name: "manual pending",
			item: exercise{Evaluation: evaluationSpec{Kind: evaluationManual}},
			response: "plausible answer",
			want:     "pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateExercise(tt.item, tt.response)
			if result.Status != tt.want {
				t.Fatalf("expected %s, got %#v", tt.want, result)
			}
		})
	}
}
