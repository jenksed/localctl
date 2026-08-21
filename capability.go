package main

import (
	"fmt"
	"sort"
	"time"
)

const (
	capabilityRuleVersion = "v1"
	capabilityFreshDays   = 14
	capabilityAgingDays   = 45
)

type capabilityAssessment struct {
	PackID                 string         `json:"pack_id"`
	PackVersion            string         `json:"pack_version"`
	PackTitle              string         `json:"pack_title"`
	ProfileID              string         `json:"profile_id"`
	State                  string         `json:"state"`
	Strength               string         `json:"strength"`
	Freshness              string         `json:"freshness"`
	ComparableRuns         int            `json:"comparable_runs"`
	ExcludedRuns           int            `json:"excluded_runs"`
	SuccessfulInference    int            `json:"successful_inference"`
	InferenceErrors        int            `json:"inference_errors"`
	Scored                 int            `json:"scored"`
	Pass                   int            `json:"pass"`
	Fail                   int            `json:"fail"`
	ManualPending          int            `json:"manual_pending"`
	HumanGood              int            `json:"human_good"`
	HumanPartial           int            `json:"human_partial"`
	HumanBad               int            `json:"human_bad"`
	DistinctExercises      int            `json:"distinct_exercises"`
	DeterministicExercises int            `json:"deterministic_exercises"`
	Coverage               float64        `json:"coverage"`
	PassRate               float64        `json:"pass_rate"`
	InferenceSuccessRate   float64        `json:"inference_success_rate"`
	Experiments            int            `json:"experiments"`
	LatestRun              time.Time      `json:"latest_run,omitempty"`
	AgeDays                int            `json:"age_days,omitempty"`
	MedianElapsed          string         `json:"median_elapsed"`
	MedianGeneration       string         `json:"median_generation"`
	FailureKinds           map[string]int `json:"failure_kinds,omitempty"`
	SourceRunIDs           []string       `json:"source_run_ids,omitempty"`
	Notes                  []string       `json:"notes,omitempty"`
}

type modelCapabilityMap struct {
	SchemaVersion int                      `json:"schema_version"`
	RuleVersion   string                   `json:"rule_version"`
	GeneratedAt   time.Time                `json:"generated_at"`
	ModelID       string                   `json:"model_id"`
	ModelName     string                   `json:"model_name"`
	Installed     bool                     `json:"installed"`
	ArtifactSHA   string                   `json:"artifact_sha256,omitempty"`
	ProfileID     string                   `json:"profile_id"`
	Machine       machineFingerprintRecord `json:"machine"`
	Roles         []string                 `json:"candidate_roles,omitempty"`
	Assessments   []capabilityAssessment   `json:"assessments"`
}

type workloadRecommendation struct {
	ModelID          string  `json:"model_id"`
	ModelName        string  `json:"model_name"`
	State            string  `json:"state"`
	Strength         string  `json:"strength"`
	Freshness        string  `json:"freshness"`
	PassRate         float64 `json:"pass_rate"`
	Coverage         float64 `json:"coverage"`
	ComparableRuns   int     `json:"comparable_runs"`
	MedianGeneration string  `json:"median_generation"`
	Supported        bool    `json:"supported"`
}

func machineComparable(a, b machineFingerprintRecord) bool {
	if a.OS == "" || b.OS == "" || a.Architecture == "" || b.Architecture == "" {
		return false
	}
	if a.OS != b.OS || a.Architecture != b.Architecture {
		return false
	}
	if a.Chip != "" && b.Chip != "" && a.Chip != b.Chip {
		return false
	}
	if a.MemoryBytes > 0 && b.MemoryBytes > 0 && a.MemoryBytes != b.MemoryBytes {
		return false
	}
	return true
}

func deterministicExerciseCount(pack packManifest) int {
	count := 0
	for _, item := range packExercises(pack) {
		if item.Evaluation.Kind != evaluationManual {
			count++
		}
	}
	return count
}

func deriveCapabilityAssessment(pack packManifest, profile profileDefinition, machine machineFingerprintRecord, artifactSHA string, now time.Time, records []runObservation, judgments map[string]humanJudgment) capabilityAssessment {
	assessment := capabilityAssessment{
		PackID:                 pack.ID,
		PackVersion:            pack.Version,
		PackTitle:              pack.Title,
		ProfileID:              profile.ID,
		DeterministicExercises: deterministicExerciseCount(pack),
		FailureKinds:           map[string]int{},
	}
	exercises := map[string]bool{}
	experiments := map[string]bool{}
	var elapsed []int64
	var speeds []float64

	for _, record := range records {
		if !capabilityRecordComparable(record, pack, profile, machine, artifactSHA) {
			assessment.ExcludedRuns++
			continue
		}
		assessment.ComparableRuns++
		assessment.SourceRunIDs = append(assessment.SourceRunIDs, record.RunID)
		if record.Result.Status == "succeeded" {
			assessment.SuccessfulInference++
			elapsed = append(elapsed, record.Result.ElapsedMS)
			if record.Result.GenerationTokensSec > 0 {
				speeds = append(speeds, record.Result.GenerationTokensSec)
			}
		} else {
			assessment.InferenceErrors++
			assessment.FailureKinds["inference_error"]++
		}
		if record.Experiment.ID != "" {
			experiments[record.Experiment.ID] = true
		}
		if assessment.LatestRun.IsZero() || record.StartedAt.After(assessment.LatestRun) {
			assessment.LatestRun = record.StartedAt
		}
		switch record.Evaluation.Status {
		case "pass":
			assessment.Pass++
			assessment.Scored++
			exercises[record.Exercise.ID] = true
		case "fail":
			assessment.Fail++
			assessment.Scored++
			exercises[record.Exercise.ID] = true
			kind := record.Evaluation.FailureKind
			if kind == "" {
				kind = "unclassified"
			}
			assessment.FailureKinds[kind]++
		case "pending":
			assessment.ManualPending++
		}
		if judgment, ok := judgments[record.RunID]; ok {
			switch judgment.Verdict {
			case "good":
				assessment.HumanGood++
			case "partial":
				assessment.HumanPartial++
			case "bad":
				assessment.HumanBad++
			}
		}
	}

	sort.Strings(assessment.SourceRunIDs)
	assessment.DistinctExercises = len(exercises)
	assessment.Experiments = len(experiments)
	if assessment.DeterministicExercises > 0 {
		assessment.Coverage = float64(assessment.DistinctExercises) / float64(assessment.DeterministicExercises)
		if assessment.Coverage > 1 {
			assessment.Coverage = 1
		}
	}
	if assessment.Scored > 0 {
		assessment.PassRate = float64(assessment.Pass) / float64(assessment.Scored)
	}
	if assessment.ComparableRuns > 0 {
		assessment.InferenceSuccessRate = float64(assessment.SuccessfulInference) / float64(assessment.ComparableRuns)
	}
	assessment.MedianElapsed = medianDuration(elapsed)
	assessment.MedianGeneration = medianSpeed(speeds)
	assessment.Freshness, assessment.AgeDays = capabilityFreshness(now, assessment.LatestRun)
	assessment.Strength = capabilityStrength(assessment)
	assessment.State = capabilityState(assessment)
	assessment.Notes = capabilityNotes(assessment)
	return assessment
}

func capabilityRecordComparable(record runObservation, pack packManifest, profile profileDefinition, machine machineFingerprintRecord, artifactSHA string) bool {
	if record.SchemaVersion < 3 || record.Input.Class == "private" {
		return false
	}
	if record.Pack.ID != pack.ID || record.Pack.Version != pack.Version {
		return false
	}
	if record.Profile.ID != profile.ID {
		return false
	}
	if !machineComparable(record.Machine, machine) {
		return false
	}
	if artifactSHA != "" && record.Model.SHA256 != artifactSHA {
		return false
	}
	return true
}

func capabilityFreshness(now, latest time.Time) (string, int) {
	if latest.IsZero() {
		return "UNKNOWN", 0
	}
	age := now.Sub(latest)
	if age < 0 {
		age = 0
	}
	days := int(age.Hours() / 24)
	switch {
	case days <= capabilityFreshDays:
		return "CURRENT", days
	case days <= capabilityAgingDays:
		return "AGING", days
	default:
		return "STALE", days
	}
}

func capabilityStrength(a capabilityAssessment) string {
	if a.Scored == 0 {
		if a.ManualPending > 0 || a.HumanGood+a.HumanPartial+a.HumanBad > 0 {
			return "HUMAN_ONLY"
		}
		return "NONE"
	}
	if a.Scored < 3 || a.Coverage < 0.25 {
		return "EARLY"
	}
	if a.Scored < 8 || a.Coverage < 0.60 {
		return "DEVELOPING"
	}
	if a.Coverage >= 0.90 && a.Experiments >= 2 {
		return "STRONG"
	}
	return "SUPPORTED"
}

func capabilityState(a capabilityAssessment) string {
	if a.ComparableRuns == 0 {
		return "UNKNOWN"
	}
	if a.Freshness == "STALE" {
		return "STALE"
	}
	if a.Scored == 0 {
		if a.ManualPending > 0 || a.HumanGood+a.HumanPartial+a.HumanBad > 0 {
			return "REVIEW_REQUIRED"
		}
		return "UNKNOWN"
	}
	if a.InferenceSuccessRate < 0.80 {
		return "RUNTIME_UNRELIABLE"
	}
	if a.PassRate < 0.60 {
		return "WEAK"
	}
	if a.PassRate < 0.85 {
		return "MIXED"
	}
	if a.Strength == "STRONG" && a.PassRate >= 0.95 && a.Freshness == "CURRENT" {
		return "STRONG"
	}
	if a.Strength == "SUPPORTED" || a.Strength == "STRONG" {
		return "SUPPORTED"
	}
	return "PROMISING"
}

func capabilityNotes(a capabilityAssessment) []string {
	var notes []string
	if a.ExcludedRuns > 0 {
		notes = append(notes, fmt.Sprintf("%d historical runs excluded because they did not match the current schema/pack/profile/machine/artifact boundary", a.ExcludedRuns))
	}
	if a.Freshness == "AGING" {
		notes = append(notes, "evidence is aging; re-test before treating it as current")
	}
	if a.Freshness == "STALE" {
		notes = append(notes, "evidence is stale under the v0.4 freshness heuristic")
	}
	if a.ManualPending > 0 {
		notes = append(notes, fmt.Sprintf("%d manual outputs still need human judgment", a.ManualPending))
	}
	if a.HumanBad > 0 {
		notes = append(notes, fmt.Sprintf("%d human-reviewed runs were judged bad", a.HumanBad))
	}
	return notes
}

func loadJudgmentsFor(records []runObservation) map[string]humanJudgment {
	result := map[string]humanJudgment{}
	for _, record := range records {
		_, runDir, err := findObservation(record.RunID)
		if err != nil {
			continue
		}
		judgment, err := loadJudgment(runDir)
		if err == nil && judgment != nil {
			result[record.RunID] = *judgment
		}
	}
	return result
}

func buildModelCapabilityMap(reference, profileRef string, now time.Time) (modelCapabilityMap, error) {
	profile, err := resolveProfile(profileRef)
	if err != nil {
		return modelCapabilityMap{}, err
	}
	modelID, modelName, records, err := historicalModelRecords(reference)
	if err != nil {
		return modelCapabilityMap{}, err
	}
	result := modelCapabilityMap{
		SchemaVersion: 1,
		RuleVersion:   capabilityRuleVersion,
		GeneratedAt:   now,
		ModelID:       modelID,
		ModelName:     modelName,
		ProfileID:     profile.ID,
		Machine:       machineFingerprint(),
	}
	artifactSHA := ""
	if model, resolveErr := resolveModel(reference); resolveErr == nil {
		result.Installed = true
		result.ModelID = model.ID
		result.ModelName = model.Name
		if digest, digestErr := cachedArtifactSHA256(model); digestErr == nil {
			artifactSHA = digest
			result.ArtifactSHA = digest
		}
	}
	judgments := loadJudgmentsFor(records)
	for _, pack := range listPacks() {
		assessment := deriveCapabilityAssessment(pack, profile, result.Machine, artifactSHA, now, records, judgments)
		result.Assessments = append(result.Assessments, assessment)
	}
	result.Roles = candidateRoles(result.Assessments)
	return result, nil
}

func candidateRoles(assessments []capabilityAssessment) []string {
	roleByPack := map[string]string{
		"developer-core":           "developer helper",
		"linux-investigation":      "Linux investigator",
		"docker-investigation":     "Docker investigator",
		"kubernetes-investigation": "Kubernetes investigator",
		"writing-summarization":    "technical writing helper",
		"structured-output":        "structured-output worker",
		"reasoning-analysis":       "bounded reasoning helper",
	}
	var roles []string
	for _, assessment := range assessments {
		if assessment.Freshness == "STALE" || (assessment.State != "SUPPORTED" && assessment.State != "STRONG") {
			continue
		}
		if role := roleByPack[assessment.PackID]; role != "" {
			roles = append(roles, role)
		}
	}
	sort.Strings(roles)
	return roles
}

func assessmentForPack(capabilities modelCapabilityMap, packID string) (capabilityAssessment, bool) {
	for _, assessment := range capabilities.Assessments {
		if assessment.PackID == packID {
			return assessment, true
		}
	}
	return capabilityAssessment{}, false
}

func recommendationStateRank(state string) int {
	switch state {
	case "STRONG":
		return 7
	case "SUPPORTED":
		return 6
	case "PROMISING":
		return 5
	case "MIXED":
		return 4
	case "WEAK":
		return 3
	case "RUNTIME_UNRELIABLE":
		return 2
	case "REVIEW_REQUIRED":
		return 1
	case "STALE":
		return 0
	default:
		return -1
	}
}

func recommendationStrengthRank(strength string) int {
	switch strength {
	case "STRONG":
		return 4
	case "SUPPORTED":
		return 3
	case "DEVELOPING":
		return 2
	case "EARLY":
		return 1
	default:
		return 0
	}
}

func buildRecommendations(pack packManifest, profile profileDefinition, now time.Time) ([]workloadRecommendation, error) {
	models, err := discoverModels()
	if err != nil {
		return nil, err
	}
	var recommendations []workloadRecommendation
	for _, model := range models {
		capabilities, buildErr := buildModelCapabilityMap(model.ID, profile.ID, now)
		if buildErr != nil {
			continue
		}
		assessment, ok := assessmentForPack(capabilities, pack.ID)
		if !ok {
			continue
		}
		recommendations = append(recommendations, workloadRecommendation{
			ModelID:          model.ID,
			ModelName:        model.Name,
			State:            assessment.State,
			Strength:         assessment.Strength,
			Freshness:        assessment.Freshness,
			PassRate:         assessment.PassRate,
			Coverage:         assessment.Coverage,
			ComparableRuns:   assessment.ComparableRuns,
			MedianGeneration: assessment.MedianGeneration,
			Supported:        assessment.State == "SUPPORTED" || assessment.State == "STRONG",
		})
	}
	sort.SliceStable(recommendations, func(i, j int) bool {
		a, b := recommendations[i], recommendations[j]
		if recommendationStateRank(a.State) != recommendationStateRank(b.State) {
			return recommendationStateRank(a.State) > recommendationStateRank(b.State)
		}
		if recommendationStrengthRank(a.Strength) != recommendationStrengthRank(b.Strength) {
			return recommendationStrengthRank(a.Strength) > recommendationStrengthRank(b.Strength)
		}
		if a.PassRate != b.PassRate {
			return a.PassRate > b.PassRate
		}
		if a.Coverage != b.Coverage {
			return a.Coverage > b.Coverage
		}
		return a.ModelName < b.ModelName
	})
	return recommendations, nil
}

func requalificationPriority(state, freshness string) int {
	if freshness == "STALE" {
		return 100
	}
	switch state {
	case "UNKNOWN":
		return 90
	case "RUNTIME_UNRELIABLE":
		return 85
	case "WEAK":
		return 80
	case "MIXED":
		return 70
	case "REVIEW_REQUIRED":
		return 60
	case "PROMISING":
		return 50
	case "SUPPORTED":
		if freshness == "AGING" {
			return 40
		}
	case "STRONG":
		if freshness == "AGING" {
			return 30
		}
	}
	return 0
}
