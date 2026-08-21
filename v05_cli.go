package main

import (
	"bytes"
	"fmt"
	"io"
)

func runV05Lab(stdout, stderr io.Writer) int {
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

	fmt.Fprintf(stdout, "LocalCTL %s — Learn What Matters Next\n\n", localctlVersion)
	fmt.Fprintln(stdout, "LEARN WHAT YOUR SYSTEM CAN DO")
	fmt.Fprintln(stdout, "LocalCTL learns by testing what this machine and its models actually demonstrate.")
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Machine   %s/%s", machine.OS, machine.Architecture)
	if machine.Chip != "" {
		fmt.Fprintf(stdout, " · %s", machine.Chip)
	}
	if machine.MemoryBytes > 0 {
		fmt.Fprintf(stdout, " · %s", formatBytes(machine.MemoryBytes))
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Models    %d installed\n", len(models))
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
	fmt.Fprintln(stdout, "WHAT SHOULD THE LAB LEARN NEXT?")
	switch {
	case len(models) == 0:
		fmt.Fprintln(stdout, "  No installed GGUF models are visible yet.")
		fmt.Fprintln(stdout, "  Start with: localctl radar")
	case len(records) == 0:
		fmt.Fprintf(stdout, "  Start by characterizing %s\n", models[0].ID)
		fmt.Fprintf(stdout, "  Run: localctl audition %s\n", models[0].ID)
	default:
		fmt.Fprintln(stdout, "  Ask the Opportunity Engine to combine local evidence gaps with current ecosystem candidates.")
		fmt.Fprintln(stdout, "  Run: localctl next")
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "MODEL RADAR")
	fmt.Fprintln(stdout, "  localctl radar             find current Hugging Face candidates screened for this machine")
	fmt.Fprintln(stdout, "  localctl radar criteria    make the screen stricter, looser, or specialist-focused")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Evidence remains the authority for capability claims. External metadata only gives the lab new questions to test.")
	return 0
}
