package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestReleaseVersion(t *testing.T) {
	if localctlVersion != "0.5.0" {
		t.Fatalf("expected v0.5.0, got %s", localctlVersion)
	}
}

func TestV03PackShape(t *testing.T) {
	want := map[string]int{
		"core-baseline":            18,
		"linux-investigation":      34,
		"docker-investigation":     33,
		"kubernetes-investigation": 33,
	}
	for packID, count := range want {
		pack, err := findPack(packID)
		if err != nil {
			t.Fatalf("find %s: %v", packID, err)
		}
		if got := len(packExercises(pack)); got != count {
			t.Fatalf("%s: expected %d exercises, got %d", packID, count, got)
		}
	}
}

func TestRepeatabilityClass(t *testing.T) {
	tests := []struct {
		pass int
		fail int
		want string
	}{
		{3, 0, "STABLE_PASS"},
		{0, 3, "STABLE_FAIL"},
		{8, 2, "USUALLY_PASS"},
		{2, 8, "USUALLY_FAIL"},
		{2, 2, "FLAKY"},
		{0, 0, "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := repeatabilityClass(tt.pass, tt.fail); got != tt.want {
			t.Fatalf("repeatabilityClass(%d,%d): want %s, got %s", tt.pass, tt.fail, tt.want, got)
		}
	}
}

func TestEvidenceCoverage(t *testing.T) {
	tests := []struct {
		runs int
		want string
	}{
		{0, "NONE"},
		{1, "EARLY"},
		{5, "MODERATE"},
		{15, "STRONG"},
		{40, "VERY_STRONG"},
	}
	for _, tt := range tests {
		if got := evidenceCoverage(tt.runs); got != tt.want {
			t.Fatalf("evidenceCoverage(%d): want %s, got %s", tt.runs, tt.want, got)
		}
	}
}

func TestQuantizationFromName(t *testing.T) {
	if got := quantizationFromName("Qwen3.5-9B-Q4_K_M.gguf"); got != "Q4_K_M" {
		t.Fatalf("unexpected quantization: %q", got)
	}
}

func TestV03LabIsUsefulWithoutInstalledModels(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runV03Lab(&stdout, &stderr); code != 0 {
		t.Fatalf("lab failed: %d: %s", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"Find the Edges", "0 historical runs", "localctl audition <model>", "localctl explore"} {
		if !strings.Contains(output, want) {
			t.Fatalf("lab output missing %q:\n%s", want, output)
		}
	}
}

func TestBuiltInProfiles(t *testing.T) {
	profiles := map[string]profileDefinition{}
	for _, profile := range builtInProfiles {
		profiles[profile.ID] = profile
	}
	for _, id := range []string{"default", "fast", "long-context"} {
		if _, ok := profiles[id]; !ok {
			t.Fatalf("missing built-in profile %s", id)
		}
	}
	if profiles["long-context"].Context <= profiles["default"].Context {
		t.Fatal("long-context profile must actually increase context")
	}
}
