package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func runCheck(stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, "LocalCTL lab check")
	fmt.Fprintln(stdout)

	if _, err := os.Stat(llamaServerPath); err != nil {
		fmt.Fprintf(stdout, "llama-server  missing  %s\n", llamaServerPath)
	} else {
		fmt.Fprintf(stdout, "llama-server  ready    %s\n", llamaServerPath)
	}

	models, err := discoverModels()
	if err != nil {
		fmt.Fprintf(stderr, "model discovery failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "GGUF models    %d found\n", len(models))

	state, err := readRuntimeState()
	if err == nil {
		var statusOut bytes.Buffer
		var statusErr bytes.Buffer
		if runtimeStatus(runtimeURL, &statusOut, &statusErr) == 0 {
			fmt.Fprintf(stdout, "runtime        ready    PID %d  %s\n", state.PID, state.ModelID)
		} else {
			fmt.Fprintf(stdout, "runtime        stale    PID %d recorded but not ready\n", state.PID)
		}
	} else {
		fmt.Fprintln(stdout, "runtime        stopped")
	}

	fmt.Fprintln(stdout)
	if len(models) == 0 {
		fmt.Fprintln(stdout, "Next: place a GGUF model under ~/.lmstudio/models")
	} else {
		fmt.Fprintln(stdout, "Next: localctl models")
	}
	return 0
}

func runModels(stdout, stderr io.Writer) int {
	models, err := discoverModels()
	if err != nil {
		fmt.Fprintf(stderr, "model discovery failed: %v\n", err)
		return 1
	}
	if len(models) == 0 {
		fmt.Fprintln(stdout, "no GGUF models found under ~/.lmstudio/models")
		return 0
	}

	fmt.Fprintln(stdout, "MODEL ID                                      SIZE       FILE")
	for _, model := range models {
		fmt.Fprintf(stdout, "%-45s %-10s %s\n", model.ID, formatBytes(model.Size), model.Name)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "You may use an unambiguous substring such as 'granite' or 'ministral' as a model reference.")
	return 0
}

func runExercises(args []string, stdout, stderr io.Writer) int {
	category := ""
	includeExtended := false
	if len(args) >= 3 {
		if args[2] == "--all" {
			includeExtended = true
		} else {
			category = args[2]
		}
	}
	if len(args) >= 4 && args[3] == "--all" {
		includeExtended = true
	}

	items := exercisesForCategory(category, includeExtended)
	if len(items) == 0 {
		fmt.Fprintf(stderr, "no exercises found for category %q\n", category)
		fmt.Fprintf(stderr, "categories: %s\n", strings.Join(exerciseCategories(), ", "))
		return 1
	}

	fmt.Fprintln(stdout, "ID                              CATEGORY      LEVEL   EVAL           TITLE")
	for _, item := range items {
		fmt.Fprintf(stdout, "%-31s %-13s %-7s %-14s %s\n", item.ID, item.Category, item.Difficulty, item.Evaluation.Kind, item.Title)
	}
	if !includeExtended {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Showing the core baseline. Use 'localctl exercises --all' for the extended catalog.")
	}
	return 0
}

func runExercise(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl exercise <show|run> ...")
		return 1
	}

	switch args[2] {
	case "show":
		if len(args) < 4 {
			fmt.Fprintln(stderr, "usage: localctl exercise show <exercise-id>")
			return 1
		}
		item, ok := findExercise(args[3])
		if !ok {
			fmt.Fprintf(stderr, "unknown exercise: %s\n", args[3])
			return 1
		}
		fmt.Fprintf(stdout, "%s — %s\n", item.ID, item.Title)
		fmt.Fprintf(stdout, "category: %s\n", item.Category)
		fmt.Fprintf(stdout, "difficulty: %s\n", item.Difficulty)
		fmt.Fprintf(stdout, "evaluation: %s\n", item.Evaluation.Kind)
		fmt.Fprintf(stdout, "description: %s\n\n", item.Description)
		fmt.Fprintln(stdout, item.Prompt)
		return 0

	case "run":
		if len(args) < 4 {
			fmt.Fprintln(stderr, "usage: localctl exercise run <exercise-id> [model]")
			return 1
		}
		item, ok := findExercise(args[3])
		if !ok {
			fmt.Fprintf(stderr, "unknown exercise: %s\n", args[3])
			return 1
		}
		modelRef := ""
		if len(args) >= 5 {
			modelRef = args[4]
		}
		return executeSingleExercise(item, modelRef, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "unknown exercise command: %s\n", args[2])
		return 1
	}
}

func runTry(args []string, stdout, stderr io.Writer) int {
	if len(args) < 4 {
		fmt.Fprintln(stderr, "usage: localctl try <model> <prompt>")
		return 1
	}

	modelRef := args[2]
	prompt := strings.Join(args[3:], " ")
	item := exercise{
		ID:          "freeform",
		Title:       "Freeform prompt",
		Category:    "freeform",
		Difficulty:  "unscored",
		Description: "A learner-supplied prompt. The observation is saved; correctness requires later judgment.",
		Prompt:      prompt,
		Evaluation:  evaluationSpec{Kind: evaluationManual},
	}
	return executeSingleExercise(item, modelRef, stdout, stderr)
}

func executeSingleExercise(item exercise, modelRef string, stdout, stderr io.Writer) int {
	model, err := modelForExecution(modelRef)
	if err != nil {
		fmt.Fprintf(stderr, "model resolution failed: %v\n", err)
		return 1
	}

	started, exitCode := ensureRuntimeForModel(model, stdout, stderr)
	if exitCode != 0 {
		return exitCode
	}
	if started {
		defer func() {
			var stopOut bytes.Buffer
			var stopErr bytes.Buffer
			_ = runtimeStop(&stopOut, &stopErr)
		}()
	}

	fmt.Fprintf(stdout, "\n%s — %s\n", item.ID, item.Title)
	fmt.Fprintf(stdout, "model: %s\n\n", model.Name)

	startedAt := time.Now()
	outcome, inferenceErr := performInference(runtimeURL, filepath.Base(model.Path), item.Prompt)
	record, _, persistErr := persistObservation(item, model, startedAt, outcome, inferenceErr)
	if persistErr != nil {
		fmt.Fprintf(stderr, "could not save run evidence: %v\n", persistErr)
		return 1
	}
	if inferenceErr != nil {
		fmt.Fprintf(stderr, "inference failed: %v\n", inferenceErr)
		fmt.Fprintf(stderr, "saved: %s\n", record.RunID)
		return 1
	}

	fmt.Fprintln(stdout, outcome.Content)
	fmt.Fprintln(stdout)
	printRunSummary(stdout, record)
	return 0
}

func runBaseline(args []string, stdout, stderr io.Writer) int {
	modelRef := ""
	includeExtended := false
	category := ""

	for _, arg := range args[2:] {
		switch {
		case arg == "--all":
			includeExtended = true
		case strings.HasPrefix(arg, "--category="):
			category = strings.TrimPrefix(arg, "--category=")
		case modelRef == "":
			modelRef = arg
		default:
			fmt.Fprintf(stderr, "unexpected baseline argument: %s\n", arg)
			return 1
		}
	}

	model, err := modelForExecution(modelRef)
	if err != nil {
		fmt.Fprintf(stderr, "model resolution failed: %v\n", err)
		return 1
	}

	items := exercisesForCategory(category, includeExtended)
	if len(items) == 0 {
		fmt.Fprintf(stderr, "no exercises selected\n")
		return 1
	}

	started, exitCode := ensureRuntimeForModel(model, stdout, stderr)
	if exitCode != 0 {
		return exitCode
	}
	if started {
		defer func() {
			var stopOut bytes.Buffer
			var stopErr bytes.Buffer
			_ = runtimeStop(&stopOut, &stopErr)
		}()
	}

	fmt.Fprintf(stdout, "\nBaseline lab: %s\n", model.Name)
	fmt.Fprintf(stdout, "Exercises: %d\n\n", len(items))

	pass := 0
	fail := 0
	pending := 0
	errors := 0

	for index, item := range items {
		fmt.Fprintf(stdout, "[%d/%d] %-31s ", index+1, len(items), item.ID)
		startedAt := time.Now()
		outcome, inferenceErr := performInference(runtimeURL, filepath.Base(model.Path), item.Prompt)
		record, _, persistErr := persistObservation(item, model, startedAt, outcome, inferenceErr)
		if persistErr != nil {
			fmt.Fprintf(stdout, "ERROR evidence: %v\n", persistErr)
			errors++
			continue
		}
		if inferenceErr != nil {
			fmt.Fprintf(stdout, "ERROR %v  [%s]\n", inferenceErr, record.RunID)
			errors++
			continue
		}

		switch record.Evaluation.Status {
		case "pass":
			pass++
			fmt.Fprintf(stdout, "PASS  %s  [%s]\n", formatDuration(outcome.Elapsed), record.RunID)
		case "fail":
			fail++
			fmt.Fprintf(stdout, "FAIL  %s  [%s]\n", formatDuration(outcome.Elapsed), record.RunID)
		case "pending":
			pending++
			fmt.Fprintf(stdout, "PENDING  %s  [%s]\n", formatDuration(outcome.Elapsed), record.RunID)
		default:
			errors++
			fmt.Fprintf(stdout, "%s  [%s]\n", strings.ToUpper(record.Evaluation.Status), record.RunID)
		}
	}

	scored := pass + fail
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Baseline summary")
	fmt.Fprintf(stdout, "model:       %s\n", model.Name)
	fmt.Fprintf(stdout, "auto scored: %d\n", scored)
	fmt.Fprintf(stdout, "pass:        %d\n", pass)
	fmt.Fprintf(stdout, "fail:        %d\n", fail)
	fmt.Fprintf(stdout, "pending:     %d\n", pending)
	fmt.Fprintf(stdout, "errors:      %d\n", errors)
	if scored > 0 {
		fmt.Fprintf(stdout, "pass rate:   %.1f%%\n", 100*float64(pass)/float64(scored))
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Next: run the same baseline with another model, then use 'localctl compare <model-a> <model-b>'.")

	if errors > 0 {
		return 1
	}
	return 0
}

func modelForExecution(reference string) (modelArtifact, error) {
	if reference != "" {
		return resolveModel(reference)
	}

	state, err := readRuntimeState()
	if err == nil {
		info, statErr := os.Stat(state.Model)
		if statErr != nil {
			return modelArtifact{}, statErr
		}
		return modelArtifact{
			ID:   modelSlug(strings.TrimSuffix(filepath.Base(state.Model), filepath.Ext(state.Model))),
			Name: filepath.Base(state.Model),
			Path: state.Model,
			Size: info.Size(),
		}, nil
	}

	return resolveModel("")
}

func ensureRuntimeForModel(model modelArtifact, stdout, stderr io.Writer) (bool, int) {
	state, err := readRuntimeState()
	if err == nil {
		if state.Model != model.Path {
			fmt.Fprintf(stderr, "managed runtime already uses %s; stop it before running %s\n", filepath.Base(state.Model), model.Name)
			return false, 1
		}
		var statusOut bytes.Buffer
		var statusErr bytes.Buffer
		if runtimeStatus(runtimeURL, &statusOut, &statusErr) == 0 {
			return false, 0
		}
		fmt.Fprintln(stderr, "managed runtime state exists but runtime is not ready; run 'localctl runtime stop' to reconcile it")
		return false, 1
	}

	if code := runtimeStartModel(model.Path, stdout, stderr); code != 0 {
		return false, code
	}
	return true, 0
}

func printRunSummary(stdout io.Writer, record runObservation) {
	fmt.Fprintln(stdout, "What happened")
	fmt.Fprintf(stdout, "result:      %s\n", record.Result.Status)
	fmt.Fprintf(stdout, "finish:      %s\n", valueOrUnknown(record.Result.FinishReason))
	fmt.Fprintf(stdout, "elapsed:     %s\n", time.Duration(record.Result.ElapsedMS)*time.Millisecond)
	if record.Result.PromptTokens > 0 || record.Result.CompletionTokens > 0 {
		fmt.Fprintf(stdout, "tokens:      %d prompt / %d completion\n", record.Result.PromptTokens, record.Result.CompletionTokens)
	}
	if record.Result.GenerationTokensSec > 0 {
		fmt.Fprintf(stdout, "generation:  %.1f tok/s\n", record.Result.GenerationTokensSec)
	}
	fmt.Fprintf(stdout, "evaluation:  %s\n", record.Evaluation.Status)
	if record.Evaluation.Detail != "" {
		fmt.Fprintf(stdout, "detail:      %s\n", record.Evaluation.Detail)
	}
	fmt.Fprintf(stdout, "saved:       %s\n", record.RunID)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Remember: successful inference is evidence of execution, not proof that a human-judged answer is correct.")
}

func runRuns(stdout, stderr io.Writer) int {
	records, err := listObservations()
	if err != nil {
		fmt.Fprintf(stderr, "could not list runs: %v\n", err)
		return 1
	}
	if len(records) == 0 {
		fmt.Fprintln(stdout, "no saved runs yet")
		return 0
	}

	fmt.Fprintln(stdout, "RUN ID                                  MODEL                    EXERCISE                       RESULT")
	limit := len(records)
	if limit > 20 {
		limit = 20
	}
	for _, record := range records[:limit] {
		fmt.Fprintf(stdout, "%-39s %-24s %-30s %s/%s\n", record.RunID, shorten(record.Model.Name, 24), shorten(record.Exercise.ID, 30), record.Result.Status, record.Evaluation.Status)
	}
	return 0
}

func runShow(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl show <run-id>")
		return 1
	}

	record, runDir, err := findObservation(args[2])
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	prompt, _ := os.ReadFile(filepath.Join(runDir, "prompt.txt"))
	response, _ := os.ReadFile(filepath.Join(runDir, "response.txt"))
	judgment, _ := loadJudgment(runDir)

	fmt.Fprintf(stdout, "run: %s\n", record.RunID)
	fmt.Fprintf(stdout, "when: %s\n", record.StartedAt.Format(time.RFC3339))
	fmt.Fprintf(stdout, "model: %s\n", record.Model.Name)
	fmt.Fprintf(stdout, "exercise: %s — %s\n", record.Exercise.ID, record.Exercise.Title)
	fmt.Fprintf(stdout, "result: %s / evaluation %s\n", record.Result.Status, record.Evaluation.Status)
	fmt.Fprintf(stdout, "finish: %s\n", valueOrUnknown(record.Result.FinishReason))
	fmt.Fprintf(stdout, "elapsed: %d ms\n", record.Result.ElapsedMS)
	if judgment != nil {
		fmt.Fprintf(stdout, "judgment: %s", judgment.Verdict)
		if judgment.Reason != "" {
			fmt.Fprintf(stdout, " — %s", judgment.Reason)
		}
		fmt.Fprintln(stdout)
	}
	fmt.Fprintf(stdout, "\nPROMPT\n%s\n", prompt)
	fmt.Fprintf(stdout, "\nRESPONSE\n%s\n", response)
	return 0
}

func runJudge(args []string, stdout, stderr io.Writer) int {
	if len(args) < 4 {
		fmt.Fprintln(stderr, "usage: localctl judge <run-id> <good|partial|bad> [reason]")
		return 1
	}
	verdict := strings.ToLower(args[3])
	if verdict != "good" && verdict != "partial" && verdict != "bad" {
		fmt.Fprintln(stderr, "verdict must be good, partial, or bad")
		return 1
	}
	reason := ""
	if len(args) > 4 {
		reason = strings.Join(args[4:], " ")
	}
	judgment, err := saveJudgment(args[2], verdict, reason)
	if err != nil {
		fmt.Fprintf(stderr, "could not save judgment: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "judged %s: %s\n", judgment.RunID, judgment.Verdict)
	return 0
}

type comparisonStats struct {
	Model      modelArtifact
	Runs       int
	Scored     int
	Pass       int
	Fail       int
	Pending    int
	Errors     int
	ElapsedMS  []int64
	Speeds     []float64
	ByCategory map[string][2]int
}

func runCompare(args []string, stdout, stderr io.Writer) int {
	if len(args) < 4 {
		fmt.Fprintln(stderr, "usage: localctl compare <model-a> <model-b>")
		return 1
	}

	modelA, err := resolveModel(args[2])
	if err != nil {
		fmt.Fprintf(stderr, "model A: %v\n", err)
		return 1
	}
	modelB, err := resolveModel(args[3])
	if err != nil {
		fmt.Fprintf(stderr, "model B: %v\n", err)
		return 1
	}

	records, err := listObservations()
	if err != nil {
		fmt.Fprintf(stderr, "could not read evidence: %v\n", err)
		return 1
	}

	a := aggregateComparison(modelA, records)
	b := aggregateComparison(modelB, records)

	fmt.Fprintln(stdout, "Saved evidence comparison")
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "%-24s %-14s %-14s\n", "", shorten(modelA.Name, 14), shorten(modelB.Name, 14))
	fmt.Fprintf(stdout, "%-24s %-14d %-14d\n", "runs", a.Runs, b.Runs)
	fmt.Fprintf(stdout, "%-24s %-14d %-14d\n", "auto-scored", a.Scored, b.Scored)
	fmt.Fprintf(stdout, "%-24s %-14s %-14s\n", "pass rate", percent(a.Pass, a.Scored), percent(b.Pass, b.Scored))
	fmt.Fprintf(stdout, "%-24s %-14d %-14d\n", "manual pending", a.Pending, b.Pending)
	fmt.Fprintf(stdout, "%-24s %-14s %-14s\n", "median elapsed", medianDuration(a.ElapsedMS), medianDuration(b.ElapsedMS))
	fmt.Fprintf(stdout, "%-24s %-14s %-14s\n", "median generation", medianSpeed(a.Speeds), medianSpeed(b.Speeds))

	categories := map[string]bool{}
	for category := range a.ByCategory {
		categories[category] = true
	}
	for category := range b.ByCategory {
		categories[category] = true
	}
	var names []string
	for category := range categories {
		names = append(names, category)
	}
	sort.Strings(names)
	if len(names) > 0 {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Auto-scored pass rate by category")
		for _, category := range names {
			ac := a.ByCategory[category]
			bc := b.ByCategory[category]
			fmt.Fprintf(stdout, "%-24s %-14s %-14s\n", category, percent(ac[0], ac[1]), percent(bc[0], bc[1]))
		}
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "These are observed results from saved runs, not a universal model ranking.")
	return 0
}

func aggregateComparison(model modelArtifact, records []runObservation) comparisonStats {
	stats := comparisonStats{Model: model, ByCategory: map[string][2]int{}}
	for _, record := range records {
		if record.Model.Path != model.Path && record.Model.ID != model.ID && record.Model.Name != model.Name {
			continue
		}
		stats.Runs++
		if record.Result.Status != "succeeded" {
			stats.Errors++
			continue
		}
		stats.ElapsedMS = append(stats.ElapsedMS, record.Result.ElapsedMS)
		if record.Result.GenerationTokensSec > 0 {
			stats.Speeds = append(stats.Speeds, record.Result.GenerationTokensSec)
		}
		switch record.Evaluation.Status {
		case "pass":
			stats.Pass++
			stats.Scored++
			value := stats.ByCategory[record.Exercise.Category]
			value[0]++
			value[1]++
			stats.ByCategory[record.Exercise.Category] = value
		case "fail":
			stats.Fail++
			stats.Scored++
			value := stats.ByCategory[record.Exercise.Category]
			value[1]++
			stats.ByCategory[record.Exercise.Category] = value
		case "pending":
			stats.Pending++
		}
	}
	return stats
}

func formatDuration(value time.Duration) string {
	if value < time.Second {
		return value.Round(time.Millisecond).String()
	}
	return value.Round(100 * time.Millisecond).String()
}

func valueOrUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func shorten(value string, width int) string {
	if len(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:width]
	}
	return value[:width-1] + "…"
}

func percent(part, total int) string {
	if total == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(part)/float64(total))
}

func medianDuration(values []int64) string {
	if len(values) == 0 {
		return "n/a"
	}
	copyValues := append([]int64(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	middle := len(copyValues) / 2
	var median int64
	if len(copyValues)%2 == 0 {
		median = (copyValues[middle-1] + copyValues[middle]) / 2
	} else {
		median = copyValues[middle]
	}
	return formatDuration(time.Duration(median) * time.Millisecond)
}

func medianSpeed(values []float64) string {
	if len(values) == 0 {
		return "n/a"
	}
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	middle := len(copyValues) / 2
	var median float64
	if len(copyValues)%2 == 0 {
		median = (copyValues[middle-1] + copyValues[middle]) / 2
	} else {
		median = copyValues[middle]
	}
	return fmt.Sprintf("%.1f tok/s", median)
}
