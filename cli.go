package main

import (
	"fmt"
	"io"
)

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: localctl <command>")
		return 1
	}

	switch args[1] {
	case "version":
		fmt.Fprintln(stdout, "localctl dev")
		return 0

	case "check":
		return runCheck(stdout, stderr)

	case "models":
		return runModels(stdout, stderr)

	case "explore":
		return runExplore(args, stdout, stderr)

	case "exercises":
		return runExercises(args, stdout, stderr)

	case "exercise":
		return runExercise(args, stdout, stderr)

	case "suite":
		return runSuite(args, stdout, stderr)

	case "try":
		return runTry(args, stdout, stderr)

	case "baseline":
		return runBaseline(args, stdout, stderr)

	case "runs":
		return runRuns(stdout, stderr)

	case "show":
		return runShow(args, stdout, stderr)

	case "judge":
		return runJudge(args, stdout, stderr)

	case "compare":
		return runCompare(args, stdout, stderr)

	case "insights":
		return runInsights(args, stdout, stderr)

	case "evidence":
		return runEvidence(args, stdout, stderr)

	case "runtime":
		return runRuntime(args, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[1])
		return 1
	}
}

func runRuntime(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl runtime <start|stop|status|inspect|infer>")
		return 1
	}

	switch args[2] {
	case "status":
		return runtimeStatus(runtimeURL, stdout, stderr)

	case "inspect":
		return runtimeInspect(runtimeURL, stdout, stderr)

	case "infer":
		if len(args) < 4 {
			fmt.Fprintln(stderr, "usage: localctl runtime infer <prompt>")
			return 1
		}

		return runtimeInfer(runtimeURL, args[3], stdout, stderr)

	case "start":
		if len(args) >= 4 {
			model, err := resolveModel(args[3])
			if err != nil {
				fmt.Fprintf(stderr, "model resolution failed: %v\n", err)
				return 1
			}
			return runtimeStartModel(model.Path, stdout, stderr)
		}
		return runtimeStart(stdout, stderr)

	case "stop":
		return runtimeStop(stdout, stderr)

	default:
		fmt.Fprintf(stderr, "unknown runtime command: %s\n", args[2])
		return 1
	}
}
