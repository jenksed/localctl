package main

import (
	"bytes"
	"fmt"
	"io"
	"sort"
)

func runV031Lab(stdout, stderr io.Writer) int {
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
	fmt.Fprintf(stdout, "Models    %d installed\n", len(models))
	fmt.Fprintf(stdout, "Evidence  %d saved runs\n", len(records))

	activeModelID := ""
	if state, stateErr := readRuntimeState(); stateErr == nil {
		var healthOut bytes.Buffer
		var healthErr bytes.Buffer
		if runtimeStatus(runtimeURL, &healthOut, &healthErr) == 0 {
			activeModelID = state.ModelID
			fmt.Fprintf(stdout, "Runtime   ready · %s · profile %s\n", state.ModelID, state.ProfileID)
		} else {
			fmt.Fprintf(stdout, "Runtime   stale managed state · PID %d\n", state.PID)
		}
	} else {
		fmt.Fprintln(stdout, "Runtime   stopped")
	}

	_, exploreState, exploreErr := loadExploreContext()
	if exploreErr == nil && len(exploreState.Selected) > 0 {
		fmt.Fprintf(stdout, "Audit     %d model candidate(s) selected\n", len(exploreState.Selected))
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Next")
	next := learnerNextStep(models, records, activeModelID)
	fmt.Fprintf(stdout, "  %s\n", next.Command)
	fmt.Fprintf(stdout, "  %s\n", next.Why)

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Common paths")
	fmt.Fprintln(stdout, "  localctl audition <model>          characterize one installed model")
	fmt.Fprintln(stdout, "  localctl headtohead <a> <b>        compare two models under one pack")
	fmt.Fprintln(stdout, "  localctl explore                   find candidates not already installed")
	fmt.Fprintln(stdout, "  localctl work ...                  try private real work")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Everything else: localctl help")
	return 0
}

type learnerNext struct {
	Command string
	Why     string
}

func learnerNextStep(models []modelArtifact, records []runObservation, activeModelID string) learnerNext {
	if len(models) == 0 {
		return learnerNext{Command: "localctl explore", Why: "No installed GGUF models are visible yet; choose a candidate before testing capability."}
	}

	runCounts := map[string]int{}
	for _, record := range records {
		runCounts[record.Model.ID]++
	}
	for _, model := range models {
		if runCounts[model.ID] == 0 {
			return learnerNext{Command: "localctl audition " + model.ID, Why: "This installed model has no saved LocalCTL evidence yet."}
		}
	}

	if activeModelID != "" {
		return learnerNext{Command: "localctl gaps " + activeModelID, Why: "The active model already has evidence; find the highest-value area to test next."}
	}
	return learnerNext{Command: "localctl gaps " + models[0].ID, Why: "Installed models already have evidence; find the biggest remaining gap instead of rerunning blindly."}
}

func runModelsV031(stdout, stderr io.Writer) int {
	models, err := discoverModels()
	if err != nil {
		fmt.Fprintf(stderr, "model discovery failed: %v\n", err)
		return 1
	}
	if len(models) == 0 {
		fmt.Fprintln(stdout, "No GGUF models found under ~/.lmstudio/models.")
		fmt.Fprintln(stdout, "Next: localctl explore")
		return 0
	}

	activePath := ""
	if state, stateErr := readRuntimeState(); stateErr == nil {
		var statusOut bytes.Buffer
		var statusErr bytes.Buffer
		if runtimeStatus(runtimeURL, &statusOut, &statusErr) == 0 {
			activePath = state.Model
		}
	}

	records, _ := listObservations()
	runsByModel := map[string]int{}
	for _, record := range records {
		runsByModel[record.Model.ID]++
	}

	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	fmt.Fprintln(stdout, "    MODEL ID                                      SIZE       RUNS")
	for _, model := range models {
		marker := " "
		if model.Path == activePath {
			marker = "*"
		}
		fmt.Fprintf(stdout, "%s   %-45s %-10s %d\n", marker, model.ID, formatBytes(model.Size), runsByModel[model.ID])
	}
	fmt.Fprintln(stdout)
	if activePath != "" {
		fmt.Fprintln(stdout, "* active managed runtime")
	}
	fmt.Fprintln(stdout, "Use any unambiguous part of a model ID in commands.")
	fmt.Fprintln(stdout, "New model: localctl audition <model> · Existing evidence: localctl report <model>")
	return 0
}
