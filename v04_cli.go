package main

import (
	"context"
	"fmt"
	"io"
)

func runV04Lab(stdout, stderr io.Writer) int {
	inspection, err := newLocalApplication().InspectMachine(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "could not inspect local lab: %v\n", err)
		return 1
	}
	machine := inspection.Machine

	fmt.Fprintf(stdout, "LocalCTL %s — Know the Territory\n\n", localctlVersion)
	fmt.Fprintf(stdout, "Machine   %s/%s", machine.OS, machine.Architecture)
	if machine.Chip != "" {
		fmt.Fprintf(stdout, " · %s", machine.Chip)
	}
	if machine.MemoryBytes > 0 {
		fmt.Fprintf(stdout, " · %s", formatBytes(machine.MemoryBytes))
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Models    %d discovered\n", inspection.Models)
	fmt.Fprintf(stdout, "Evidence  %d historical runs\n", inspection.EvidenceRuns)
	switch inspection.Runtime.State {
	case "ready":
		fmt.Fprintf(stdout, "Runtime   ready · %s · profile %s · PID %d\n", inspection.Runtime.ModelID, inspection.Runtime.ProfileID, inspection.Runtime.PID)
	case "recorded":
		fmt.Fprintf(stdout, "Runtime   stale managed state · PID %d\n", inspection.Runtime.PID)
	default:
		fmt.Fprintln(stdout, "Runtime   stopped")
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "OPERATE THE LAB")
	fmt.Fprintln(stdout, "  localctl lab web                       open the local web application on 127.0.0.1:7331")
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
	fmt.Fprintln(stdout, "The CLI and web UI call the same LocalCTL application operations. Derived intelligence does not authorize model execution; presentation does not become evidence or authority.")
	return 0
}
