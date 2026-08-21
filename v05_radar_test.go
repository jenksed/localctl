package main

import (
	"net/url"
	"reflect"
	"testing"
	"time"
)

func TestAutomaticRadarMaxParamsUsesConservativeMachineBudget(t *testing.T) {
	m16 := machineFingerprintRecord{MemoryBytes: 16 * 1024 * 1024 * 1024}
	if got := automaticRadarMaxParams(m16, "comfortable"); got != 13.5 {
		t.Fatalf("expected 16 GB comfortable screen to be 13.5B, got %.1fB", got)
	}
	m8 := machineFingerprintRecord{MemoryBytes: 8 * 1024 * 1024 * 1024}
	if got := automaticRadarMaxParams(m8, "comfortable"); got != 6.7 {
		t.Fatalf("expected 8 GB comfortable screen to be 6.7B, got %.1fB", got)
	}
	if got := automaticRadarMaxParams(m16, "any"); got != 0 {
		t.Fatalf("fit=any should disable the automatic parameter cap, got %.1fB", got)
	}
}

func TestParametersFromModelIDIsOnlyAHint(t *testing.T) {
	tests := map[string]float64{
		"Qwen/Qwen3.5-9B-GGUF":       9,
		"org/Gemma-4-E4B-it-GGUF":    4,
		"org/something-0.6B-GGUF":    0.6,
		"org/no-parameter-size-GGUF": 0,
	}
	for input, want := range tests {
		if got := parametersFromModelID(input); got != want {
			t.Fatalf("parametersFromModelID(%q): want %.1f, got %.1f", input, want, got)
		}
	}
}

func TestRadarLiveQueryUsesFirstClassGGUFAndParameterFilters(t *testing.T) {
	criteria := defaultRadarCriteria()
	machine := machineFingerprintRecord{MemoryBytes: 16 * 1024 * 1024 * 1024}
	endpoint, err := url.Parse(radarQueryURL(criteria, machine))
	if err != nil {
		t.Fatal(err)
	}
	query := endpoint.Query()
	if got := query.Get("filter"); got != "gguf" {
		t.Fatalf("expected first-class GGUF filter, got %q", got)
	}
	if got := query.Get("num_parameters"); got != "max:13.5B" {
		t.Fatalf("expected Hugging Face parameter filter max:13.5B, got %q", got)
	}
	if got := query.Get("search"); got != "" {
		t.Fatalf("repository-name search must not stand in for GGUF filtering, got %q", got)
	}
}

func TestRadarFitAnyDoesNotSendParameterFilter(t *testing.T) {
	criteria := defaultRadarCriteria()
	criteria.Fit = "any"
	machine := machineFingerprintRecord{MemoryBytes: 16 * 1024 * 1024 * 1024}
	endpoint, err := url.Parse(radarQueryURL(criteria, machine))
	if err != nil {
		t.Fatal(err)
	}
	if got := endpoint.Query().Get("num_parameters"); got != "" {
		t.Fatalf("fit=any should omit parameter filter, got %q", got)
	}
}

func TestRadarRanksRecencyAndFiltersSpecialists(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	criteria := defaultRadarCriteria()
	machine := machineFingerprintRecord{MemoryBytes: 16 * 1024 * 1024 * 1024}
	results := []hfModelResult{
		{ID: "example/FreshCoder-7B", LastModified: now.Add(-24 * time.Hour).Format(time.RFC3339), Downloads: 100, Tags: []string{"gguf", "code", "text-generation", "license:apache-2.0"}},
		{ID: "example/OldCoder-7B", LastModified: now.Add(-120 * 24 * time.Hour).Format(time.RFC3339), Downloads: 100, Tags: []string{"gguf", "code"}},
		{ID: "example/TinyEmbed-0.6B", LastModified: now.Add(-24 * time.Hour).Format(time.RFC3339), Downloads: 100, Tags: []string{"gguf", "embedding", "feature-extraction"}},
		{ID: "example/PlainModel-7B", LastModified: now.Add(-24 * time.Hour).Format(time.RFC3339), Downloads: 100, Tags: []string{"code"}},
	}
	got := rankRadarResults(results, criteria, machine, nil, now)
	if len(got) != 1 {
		t.Fatalf("expected only the fresh general GGUF candidate, got %#v", got)
	}
	if got[0].Repository != "example/FreshCoder-7B" {
		t.Fatalf("unexpected candidate: %#v", got[0])
	}
	if got[0].LocalStatus != "UNTESTED_LOCAL" {
		t.Fatalf("external discovery must remain untested locally, got %s", got[0].LocalStatus)
	}
	if got[0].Fit != "COMFORTABLE_SCREEN" {
		t.Fatalf("expected visible screening label, got %q", got[0].Fit)
	}
}

func TestRadarDoesNotUseRepositoryParameterHintAsFitAuthority(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	criteria := defaultRadarCriteria()
	machine := machineFingerprintRecord{MemoryBytes: 8 * 1024 * 1024 * 1024}
	// The repository name says 32B. If Hugging Face returned this from the live
	// max:6.7B query, LocalCTL must not second-guess authoritative server metadata
	// using a repository-name heuristic. The parsed value remains a display hint.
	result := hfModelResult{
		ID:           "example/WeirdName-32B-GGUF",
		LastModified: now.Add(-24 * time.Hour).Format(time.RFC3339),
		Downloads:    100,
		Tags:         []string{"gguf", "code"},
	}
	got := rankRadarResults([]hfModelResult{result}, criteria, machine, nil, now)
	if len(got) != 1 {
		t.Fatalf("repository-name parameter hint incorrectly became fit authority: %#v", got)
	}
	if got[0].ParametersB != 32 {
		t.Fatalf("expected repository-name display hint, got %.1f", got[0].ParametersB)
	}
}

func TestRadarCanExplicitlySearchForEmbeddingSpecialists(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	criteria := defaultRadarCriteria()
	criteria.Uses = []string{"embedding"}
	machine := machineFingerprintRecord{MemoryBytes: 16 * 1024 * 1024 * 1024}
	results := []hfModelResult{
		{ID: "example/TinyEmbed-0.6B", LastModified: now.Add(-24 * time.Hour).Format(time.RFC3339), Downloads: 100, Tags: []string{"gguf", "embedding", "feature-extraction", "license:apache-2.0"}},
	}
	got := rankRadarResults(results, criteria, machine, nil, now)
	if len(got) != 1 || !reflect.DeepEqual(got[0].UseMatches, []string{"embedding"}) {
		t.Fatalf("expected embedding specialist when explicitly requested, got %#v", got)
	}
}

func TestRadarLicenseMetadataFiltersAreExplicit(t *testing.T) {
	if !licensePassesFilter("apache-2.0", "permissive") {
		t.Fatal("Apache-2.0 should pass permissive metadata filter")
	}
	if licensePassesFilter("cc-by-4.0", "permissive") {
		t.Fatal("CC-BY-4.0 should not pass the narrow permissive metadata filter")
	}
	if !licensePassesFilter("cc-by-4.0", "known-commercial") {
		t.Fatal("CC-BY-4.0 should pass known-commercial metadata filter")
	}
	if licensePassesFilter("", "known-commercial") {
		t.Fatal("unknown license metadata must not pass a restrictive filter")
	}
}

func TestRadarCriteriaPersistAndNormalize(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	criteria := defaultRadarCriteria()
	criteria.Fit = "stretch"
	criteria.Uses = []string{"Review", "coding", "coding"}
	criteria.ReleaseDays = 30
	criteria.License = "permissive"
	if err := saveRadarCriteria(criteria); err != nil {
		t.Fatal(err)
	}
	got, err := loadRadarCriteria()
	if err != nil {
		t.Fatal(err)
	}
	if got.Fit != "stretch" || got.ReleaseDays != 30 || got.License != "permissive" {
		t.Fatalf("criteria did not round trip: %#v", got)
	}
	if !reflect.DeepEqual(got.Uses, []string{"coding", "review"}) {
		t.Fatalf("expected normalized uses, got %#v", got.Uses)
	}
}

func TestEcosystemOpportunityNeverBecomesRecommendation(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	candidate := radarCandidate{
		Repository:   "example/FreshCoder-7B-GGUF",
		ParametersB:  7,
		LastModified: now.Add(-24 * time.Hour).Format(time.RFC3339),
		UseMatches:   []string{"coding", "debugging", "review"},
		Fit:          "COMFORTABLE_SCREEN",
		LocalStatus:  "UNTESTED_LOCAL",
		Why:          "matches requested uses",
		Source:       "https://huggingface.co/example/FreshCoder-7B-GGUF",
	}
	opportunities := buildRadarLearningOpportunities([]radarCandidate{candidate}, now)
	if len(opportunities) != 1 {
		t.Fatalf("expected one ecosystem opportunity, got %#v", opportunities)
	}
	got := opportunities[0]
	if got.Status != "UNTESTED_LOCAL" || got.Kind != "ecosystem-candidate" {
		t.Fatalf("external candidate was over-promoted: %#v", got)
	}
	if got.NextCommand != "localctl explore select example/FreshCoder-7B-GGUF" {
		t.Fatalf("external candidate should go to audit selection, got %q", got.NextCommand)
	}
	if got.Priority >= 340 {
		t.Fatalf("ecosystem metadata must not outrank an aging supported local-evidence refresh; priority=%d", got.Priority)
	}
}
