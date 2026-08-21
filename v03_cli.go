package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type experimentSetSummary struct {
	Pass    int
	Fail    int
	Pending int
	Errors  int
	Records []runObservation
}

func ensureRuntimeForModelProfile(model modelArtifact, profile profileDefinition, stdout, stderr io.Writer) (bool, int) {
	state, err := readRuntimeState()
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(stderr, "could not read managed runtime state: %v\n", err)
			return false, 1
		}
		if code := runtimeStartModelWithProfile(model.Path, profile, stdout, stderr); code != 0 {
			return false, code
		}
		return true, 0
	}
	var statusOut bytes.Buffer
	var statusErr bytes.Buffer
	ready := runtimeStatus(runtimeURL, &statusOut, &statusErr) == 0
	if !ready {
		fmt.Fprintf(stdout, "runtime: recorded state for %s is not ready; reconciling automatically\n", state.ModelID)
		if code := reconcileLabRuntimeState(stdout, stderr); code != 0 {
			return false, code
		}
		if code := runtimeStartModelWithProfile(model.Path, profile, stdout, stderr); code != 0 {
			return false, code
		}
		return true, 0
	}
	profileMatches := state.ProfileID == profile.ID && state.Context == profile.Context && state.MaxTokens == profile.MaxTokens && state.Temperature == profile.Temperature
	if state.Model == model.Path && profileMatches {
		fmt.Fprintf(stdout, "runtime: reusing %s with profile %s (PID %d)\n", state.ModelID, profile.ID, state.PID)
		return false, 0
	}
	if state.Model == model.Path {
		fmt.Fprintf(stdout, "runtime: restarting %s for profile %s -> %s\n", state.ModelID, state.ProfileID, profile.ID)
	} else {
		fmt.Fprintf(stdout, "runtime: switching %s -> %s\n", state.ModelID, model.Name)
	}
	if code := runtimeStop(stdout, stderr); code != 0 {
		return false, code
	}
	if code := runtimeStartModelWithProfile(model.Path, profile, stdout, stderr); code != 0 {
		return false, code
	}
	return true, 0
}

func runV03Lab(stdout, stderr io.Writer) int {
	machine := machineFingerprint()
	models, err := discoverModels()
	if err != nil {
		fmt.Fprintf(stderr, "model discovery failed: %v\n", err)
		return 1
	}
	records, err := listObservations()
	if err != nil {
		fmt.Fprintf(stderr, "could not read evidence: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "LocalCTL %s — Find the Edges\n\n", localctlVersion)
	fmt.Fprintf(stdout, "Machine   %s/%s", machine.OS, machine.Architecture)
	if machine.Chip != "" {
		fmt.Fprintf(stdout, " · %s", machine.Chip)
	}
	if machine.MemoryBytes > 0 {
		fmt.Fprintf(stdout, " · %s", formatBytes(machine.MemoryBytes))
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Models    %d discovered\n", len(models))
	fmt.Fprintf(stdout, "Evidence  %d historical runs\n", len(records))
	if state, stateErr := readRuntimeState(); stateErr == nil {
		var healthOut bytes.Buffer
		var healthErr bytes.Buffer
		if runtimeStatus(runtimeURL, &healthOut, &healthErr) == 0 {
			fmt.Fprintf(stdout, "Runtime   ready · %s · profile %s · PID %d\n", state.ModelID, state.ProfileID, state.PID)
		} else {
			fmt.Fprintf(stdout, "Runtime   stale managed state · PID %d\n", state.PID)
		}
	} else {
		fmt.Fprintln(stdout, "Runtime   stopped")
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Start here")
	fmt.Fprintln(stdout, "  localctl audition <model>              guided model characterization")
	fmt.Fprintln(stdout, "  localctl missions                      real-work learning paths")
	fmt.Fprintln(stdout, "  localctl packs                         versioned capability packs")
	fmt.Fprintln(stdout, "  localctl verify <exercise> <model>     repeatability check")
	fmt.Fprintln(stdout, "  localctl report <model>                historical characterization")
	fmt.Fprintln(stdout, "  localctl gaps <model>                  highest-value evidence gaps")
	fmt.Fprintln(stdout, "  localctl explore                       candidate model radar")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Glass box")
	fmt.Fprintln(stdout, "  localctl runtime status|inspect|stop")
	fmt.Fprintln(stdout, "  localctl evidence audit")
	return 0
}

func executePackRun(pack packManifest, modelRef, profileRef string, stdout, stderr io.Writer) int {
	model, err := resolveModel(modelRef)
	if err != nil {
		fmt.Fprintf(stderr, "model resolution failed: %v\n", err)
		return 1
	}
	profile, err := resolveProfile(profileRef)
	if err != nil {
		fmt.Fprintf(stderr, "profile resolution failed: %v\n", err)
		return 1
	}
	items := packExercises(pack)
	if len(items) == 0 {
		fmt.Fprintf(stderr, "pack %s has no exercises on this build\n", pack.ID)
		return 1
	}
	if _, code := ensureRuntimeForModelProfile(model, profile, stdout, stderr); code != 0 {
		return code
	}
	session, finishSession := startSession("pack", pack.ID+" "+model.ID)
	defer finishSession()
	experiment, finishExperiment := startScopedExperiment("pack", pack.ID+"/"+pack.Version, model, profile, pack, "canonical")
	defer finishExperiment()
	fmt.Fprintf(stdout, "\nCapability pack: %s/%s\n", pack.ID, pack.Version)
	fmt.Fprintf(stdout, "model: %s\nprofile: %s\nsession: %s\nexperiment: %s\nexercises: %d\n\n", model.Name, profile.ID, session.ID, experiment.ID, len(items))
	summary := runItemsV03(items, model, profile, stdout)
	printSetSummary(stdout, summary)
	fmt.Fprintf(stdout, "\nRuntime stays ready on %s with profile %s.\n", model.Name, profile.ID)
	if summary.Errors > 0 {
		return 1
	}
	return 0
}

func runItemsV03(items []exercise, model modelArtifact, profile profileDefinition, stdout io.Writer) experimentSetSummary {
	summary := experimentSetSummary{}
	for index, item := range items {
		fmt.Fprintf(stdout, "[%3d/%3d] %-34s ", index+1, len(items), item.ID)
		record, outcome, err := runObservedItem(item, model, profile)
		if err != nil {
			fmt.Fprintf(stdout, "ERROR    %s\n", err)
			summary.Errors++
			if record.RunID != "" {
				summary.Records = append(summary.Records, record)
			}
			continue
		}
		summary.Records = append(summary.Records, record)
		switch record.Evaluation.Status {
		case "pass":
			summary.Pass++
			fmt.Fprintf(stdout, "PASS     %s\n", formatDuration(outcome.Elapsed))
		case "fail":
			summary.Fail++
			fmt.Fprintf(stdout, "FAIL     %s\n", formatDuration(outcome.Elapsed))
			fmt.Fprintf(stdout, "          %-24s %s\n", record.Evaluation.FailureKind, baselineOneLine(record.Evaluation.Detail, 130))
			fmt.Fprintf(stdout, "          saved: %s\n", record.RunID)
		case "pending":
			summary.Pending++
			fmt.Fprintf(stdout, "PENDING  %s  [%s]\n", formatDuration(outcome.Elapsed), record.RunID)
		default:
			summary.Errors++
			fmt.Fprintf(stdout, "%s\n", strings.ToUpper(record.Evaluation.Status))
		}
	}
	return summary
}

func runObservedItem(item exercise, model modelArtifact, profile profileDefinition) (runObservation, inferenceOutcome, error) {
	if scopedObservation != nil && scopedObservation.Pack.ID == "audition" {
		scopedObservation.Pack = inferredPackForExercise(item)
	}
	startedAt := time.Now()
	outcome, inferenceErr := performInferenceWithProfile(runtimeURL, model.Name, item.Prompt, profile)
	record, _, persistErr := persistObservation(item, model, startedAt, outcome, inferenceErr)
	if persistErr != nil {
		return runObservation{}, outcome, fmt.Errorf("save evidence: %w", persistErr)
	}
	if inferenceErr != nil {
		return record, outcome, inferenceErr
	}
	return record, outcome, nil
}

func printSetSummary(stdout io.Writer, summary experimentSetSummary) {
	scored := summary.Pass + summary.Fail
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Experiment summary")
	fmt.Fprintf(stdout, "auto pass: %d\nauto fail: %d\nmanual pending: %d\nerrors: %d\n", summary.Pass, summary.Fail, summary.Pending, summary.Errors)
	if scored > 0 {
		fmt.Fprintf(stdout, "auto-scored pass rate: %s\n", percent(summary.Pass, scored))
	}
}

func runAudition(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl audition <model> [--profile=default]")
		return 1
	}
	model, err := resolveModel(args[2])
	if err != nil {
		fmt.Fprintf(stderr, "model resolution failed: %v\n", err)
		return 1
	}
	profileRef := "default"
	for _, arg := range args[3:] {
		if strings.HasPrefix(arg, "--profile=") {
			profileRef = strings.TrimPrefix(arg, "--profile=")
		} else {
			fmt.Fprintf(stderr, "unexpected audition argument: %s\n", arg)
			return 1
		}
	}
	profile, err := resolveProfile(profileRef)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "LocalCTL model audition\nmodel: %s\nprofile: %s\n\n", model.Name, profile.ID)
	fmt.Fprintln(stdout, "Stage 1/5 — runtime fit")
	startedRuntime, code := ensureRuntimeForModelProfile(model, profile, stdout, stderr)
	if code != 0 {
		return code
	}
	if digest, digestErr := cachedArtifactSHA256(model); digestErr == nil {
		fmt.Fprintf(stdout, "artifact: sha256:%s\n", shorten(digest, 20))
	}
	if startedRuntime {
		fmt.Fprintln(stdout, "runtime: cold start observed above")
	} else {
		fmt.Fprintln(stdout, "runtime: warm managed runtime reused")
	}

	session, finishSession := startSession("audition", "audition "+model.ID)
	defer finishSession()
	auditionPack := packManifest{ID: "audition", Version: "v1", Title: "Adaptive audition", Purpose: "Fast characterization across useful capability edges.", DoesNotProve: "Production qualification."}
	experiment, finishExperiment := startScopedExperiment("audition", "adaptive audition", model, profile, auditionPack, "canonical")
	defer finishExperiment()
	fmt.Fprintf(stdout, "session: %s\nexperiment: %s\n\n", session.ID, experiment.ID)

	fmt.Fprintln(stdout, "Stage 2/5 — core contracts")
	core, _ := findPack("core-baseline")
	coreSummary := runItemsV03(packExercises(core), model, profile, stdout)
	allRecords := append([]runObservation(nil), coreSummary.Records...)
	errors := coreSummary.Errors

	fmt.Fprintln(stdout, "\nStage 3/5 — adaptive capability probes")
	seen := map[string]bool{}
	for _, record := range allRecords {
		seen[record.Exercise.ID] = true
	}
	probeCategories := []string{"developer", "coding", "reasoning", "analysis", "linux", "docker", "kubernetes", "summarization"}
	for _, category := range probeCategories {
		var candidates []exercise
		for _, item := range allExerciseCatalog() {
			if item.Category != category || seen[item.ID] || item.Evaluation.Kind == evaluationManual {
				continue
			}
			candidates = append(candidates, item)
		}
		if len(candidates) == 0 {
			continue
		}
		limit := 2
		if len(candidates) < limit {
			limit = len(candidates)
		}
		first := runItemsV03(candidates[:limit], model, profile, stdout)
		allRecords = append(allRecords, first.Records...)
		errors += first.Errors
		if first.Pass == limit {
			fmt.Fprintf(stdout, "          %s: early clean signal; stopped after %d probes\n", category, limit)
			continue
		}
		extraEnd := len(candidates)
		if extraEnd > 5 {
			extraEnd = 5
		}
		if extraEnd > limit {
			fmt.Fprintf(stdout, "          %s: mixed/weak signal; adding %d discriminators\n", category, extraEnd-limit)
			extra := runItemsV03(candidates[limit:extraEnd], model, profile, stdout)
			allRecords = append(allRecords, extra.Records...)
			errors += extra.Errors
		}
	}

	fmt.Fprintln(stdout, "\nStage 4/5 — repeatability candidates")
	unstableCandidates := selectRepeatabilityCandidates(allRecords)
	if len(unstableCandidates) == 0 {
		fmt.Fprintln(stdout, "No immediate deterministic disagreement to chase inside this audition.")
	} else {
		for _, item := range unstableCandidates {
			fmt.Fprintf(stdout, "suggested verify: localctl verify %s %s --profile=%s\n", item.ID, model.ID, profile.ID)
		}
	}

	fmt.Fprintln(stdout, "\nStage 5/5 — early characterization")
	printAuditionCharacterization(stdout, model, profile, allRecords)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "What this supports: an early bounded capability map for this exact model/profile/machine.")
	fmt.Fprintln(stdout, "What this does not prove: general production reliability, safe autonomy, or capability outside tested workloads.")
	fmt.Fprintf(stdout, "Useful next step: localctl gaps %s\n", model.ID)
	if errors > 0 {
		return 1
	}
	return 0
}

func selectRepeatabilityCandidates(records []runObservation) []exercise {
	failed := map[string]bool{}
	for _, record := range records {
		if record.Evaluation.Status == "fail" {
			failed[record.Exercise.ID] = true
		}
	}
	var result []exercise
	for id := range failed {
		if item, ok := findExercise(id); ok && item.Evaluation.Kind != evaluationManual {
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	if len(result) > 5 {
		result = result[:5]
	}
	return result
}

func printAuditionCharacterization(stdout io.Writer, model modelArtifact, profile profileDefinition, records []runObservation) {
	byCategory := map[string][2]int{}
	var elapsed []int64
	var speeds []float64
	failureKinds := map[string]int{}
	for _, record := range records {
		if record.Result.Status == "succeeded" {
			elapsed = append(elapsed, record.Result.ElapsedMS)
			if record.Result.GenerationTokensSec > 0 {
				speeds = append(speeds, record.Result.GenerationTokensSec)
			}
		}
		value := byCategory[record.Exercise.Category]
		switch record.Evaluation.Status {
		case "pass":
			value[0]++
			value[1]++
		case "fail":
			value[1]++
			failureKinds[record.Evaluation.FailureKind]++
		}
		byCategory[record.Exercise.Category] = value
	}
	fmt.Fprintf(stdout, "%s · profile %s\n", model.Name, profile.ID)
	var categories []string
	for category, score := range byCategory {
		if score[1] > 0 {
			categories = append(categories, category)
		}
	}
	sort.Strings(categories)
	for _, category := range categories {
		score := byCategory[category]
		fmt.Fprintf(stdout, "%-16s %3d/%-3d %s\n", category, score[0], score[1], percent(score[0], score[1]))
	}
	fmt.Fprintf(stdout, "median elapsed:    %s\n", medianDuration(elapsed))
	fmt.Fprintf(stdout, "median generation: %s\n", medianSpeed(speeds))
	if len(failureKinds) > 0 {
		fmt.Fprintln(stdout, "failure modes:")
		var keys []string
		for key := range failureKinds {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(stdout, "  %-28s %d\n", key, failureKinds[key])
		}
	}
}

func runVerify(args []string, stdout, stderr io.Writer) int {
	if len(args) < 4 {
		fmt.Fprintln(stderr, "usage: localctl verify <exercise-or-category> <model> [--runs=7] [--profile=default]")
		return 1
	}
	target := args[2]
	model, err := resolveModel(args[3])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	profileRef := "default"
	maxRuns := 7
	for _, arg := range args[4:] {
		switch {
		case strings.HasPrefix(arg, "--profile="):
			profileRef = strings.TrimPrefix(arg, "--profile=")
		case strings.HasPrefix(arg, "--runs="):
			maxRuns, err = strconv.Atoi(strings.TrimPrefix(arg, "--runs="))
			if err != nil || maxRuns < 3 || maxRuns > 20 {
				fmt.Fprintln(stderr, "--runs must be between 3 and 20")
				return 1
			}
		default:
			fmt.Fprintf(stderr, "unexpected verify argument: %s\n", arg)
			return 1
		}
	}
	profile, err := resolveProfile(profileRef)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var items []exercise
	if item, ok := findExercise(target); ok {
		if item.Evaluation.Kind == evaluationManual {
			fmt.Fprintln(stderr, "repeatability verification currently requires a deterministic exercise")
			return 1
		}
		items = []exercise{item}
	} else if knownExerciseCategory(target) {
		for _, item := range exercisesForCategory(target, true) {
			if item.Evaluation.Kind != evaluationManual {
				items = append(items, item)
			}
		}
		if len(items) > 8 {
			items = items[:8]
			fmt.Fprintln(stdout, "category verification samples the first 8 deterministic exercises; verify a specific exercise for deeper evidence")
		}
	} else {
		fmt.Fprintf(stderr, "unknown exercise or category: %s\n", target)
		return 1
	}
	if _, code := ensureRuntimeForModelProfile(model, profile, stdout, stderr); code != 0 {
		return code
	}
	session, finishSession := startSession("verify", "verify "+target+" "+model.ID)
	defer finishSession()
	pack := inferredPackForExercise(items[0])
	experiment, finishExperiment := startScopedExperiment("verify", "repeatability "+target, model, profile, pack, "canonical")
	defer finishExperiment()
	fmt.Fprintf(stdout, "\nRepeatability verification\nsession: %s\nexperiment: %s\nmodel: %s\nprofile: %s\nmax trials: %d\n\n", session.ID, experiment.ID, model.Name, profile.ID, maxRuns)
	errors := 0
	for _, item := range items {
		pass, fail := 0, 0
		trials := 0
		for trials < maxRuns {
			record, outcome, runErr := runObservedItem(item, model, profile)
			trials++
			if runErr != nil {
				errors++
				fmt.Fprintf(stdout, "%-34s trial %d ERROR %v\n", item.ID, trials, runErr)
				continue
			}
			if record.Evaluation.Status == "pass" {
				pass++
			} else if record.Evaluation.Status == "fail" {
				fail++
			}
			fmt.Fprintf(stdout, "%-34s trial %d %-4s %s\n", item.ID, trials, strings.ToUpper(record.Evaluation.Status), formatDuration(outcome.Elapsed))
			if trials >= 3 && (pass == trials || fail == trials) {
				break
			}
		}
		fmt.Fprintf(stdout, "  result: %s · %d pass / %d fail / %d trials\n", repeatabilityClass(pass, fail), pass, fail, trials)
	}
	if errors > 0 {
		return 1
	}
	return 0
}

func repeatabilityClass(pass, fail int) string {
	total := pass + fail
	if total == 0 {
		return "UNKNOWN"
	}
	if fail == 0 && total >= 3 {
		return "STABLE_PASS"
	}
	if pass == 0 && total >= 3 {
		return "STABLE_FAIL"
	}
	ratio := float64(pass) / float64(total)
	if ratio >= 0.8 {
		return "USUALLY_PASS"
	}
	if ratio <= 0.2 {
		return "USUALLY_FAIL"
	}
	return "FLAKY"
}
