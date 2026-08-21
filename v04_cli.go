package main

import (
	"bytes"
	"fmt"
	"io"
)

func runV04Lab(stdout, stderr io.Writer) int {
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

	fmt.Fprintf(stdout, "LocalCTL %s — Know the Territory\n\n", localctlVersion)
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
	fmt.Fprintln(stdout, "KNOW WHAT YOUR MAC CAN HANDLE")
	fmt.Fprintln(stdout, "  discover → test → verify → map → recommend → refresh")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Start here")
	fmt.Fprintln(stdout, "  localctl audition <model>              find a model's edges")
	fmt.Fprintln(stdout, "  localctl capability <model>            turn matching evidence into a capability map")
	fmt.Fprintln(stdout, "  localctl matrix                        see the local fleet at a glance")
	fmt.Fprintln(stdout, "  localctl recommend <pack>              rank installed models by bounded current evidence")
	fmt.Fprintln(stdout, "  localctl requalify <model>             plan the highest-value refresh work")
	fmt.Fprintln(stdout, "  localctl headtohead <a> <b>            create new controlled comparison evidence")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Evidence + intelligence")
	fmt.Fprintln(stdout, "  localctl packs                         inspect versioned workload definitions")
	fmt.Fprintln(stdout, "  localctl missions                      run intent-oriented experiment groups")
	fmt.Fprintln(stdout, "  localctl report <model>                inspect historical characterization")
	fmt.Fprintln(stdout, "  localctl gaps <model>                  find under-tested capability areas")
	fmt.Fprintln(stdout, "  localctl evidence audit                verify the durable observation corpus")
	fmt.Fprintln(stdout, "  localctl intelligence audit            verify derived claims still trace to source runs")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Glass box")
	fmt.Fprintln(stdout, "  localctl runtime status|inspect|stop")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "v0.4 intelligence is deterministic decision support. It does not authorize model execution or silently turn evidence into trust.")
	return 0
}
