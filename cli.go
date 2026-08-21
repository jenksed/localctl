package main

import (
	"fmt"
	"io"
	"strings"
)

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		return runHelp(stdout)
	}

	switch args[1] {
	case "help", "--help", "-h":
		return runHelp(stdout)
	case "version":
		fmt.Fprintf(stdout, "localctl %s\n", localctlVersion)
		return 0
	case "lab":
		fmt.Fprintln(stdout, localctlBanner)
		fmt.Fprintln(stdout)
		return runV04Lab(stdout, stderr)
	case "check":
		return runCheck(stdout, stderr)
	case "models":
		return runModelsV031(stdout, stderr)
	case "explore":
		return runExplore(args, stdout, stderr)
	case "profiles":
		return runProfiles(args, stdout, stderr)
	case "profile":
		return runProfile(args, stdout, stderr)
	case "packs":
		return runPacks(stdout, stderr)
	case "pack":
		return runPack(args, stdout, stderr)
	case "experiment":
		return runExperiment(args, stdout, stderr)
	case "missions":
		return runMissions(stdout, stderr)
	case "mission":
		return runMission(args, stdout, stderr)
	case "audition":
		return runAudition(args, stdout, stderr)
	case "headtohead":
		return runHeadToHead(args, stdout, stderr)
	case "verify":
		return runVerify(args, stdout, stderr)
	case "work":
		return runWork(args, stdout, stderr)
	case "capability":
		return runCapability(args, stdout, stderr)
	case "matrix":
		return runMatrix(args, stdout, stderr)
	case "recommend":
		return runRecommend(args, stdout, stderr)
	case "requalify":
		return runRequalify(args, stdout, stderr)
	case "intelligence":
		return runIntelligence(args, stdout, stderr)
	case "report":
		return runReport(args, stdout, stderr)
	case "gaps":
		return runGaps(args, stdout, stderr)
	case "exercises":
		return runExercises(args, stdout, stderr)
	case "exercise":
		return runExercise(args, stdout, stderr)
	case "suite":
		return runSuite(args, stdout, stderr)
	case "try":
		return runTry(args, stdout, stderr)
	case "baseline":
		return runBaselineV03(args, stdout, stderr)
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
		fmt.Fprintln(stderr, "Run 'localctl help' to see the learner-facing command map.")
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
		return runtimeInfer(runtimeURL, strings.Join(args[3:], " "), stdout, stderr)
	case "start":
		modelRef := ""
		profileRef := "default"
		for _, arg := range args[3:] {
			if strings.HasPrefix(arg, "--profile=") {
				profileRef = strings.TrimPrefix(arg, "--profile=")
			} else if modelRef == "" {
				modelRef = arg
			} else {
				fmt.Fprintf(stderr, "unexpected runtime start argument: %s\n", arg)
				return 1
			}
		}
		profile, err := resolveProfile(profileRef)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if modelRef != "" {
			model, err := resolveModel(modelRef)
			if err != nil {
				fmt.Fprintf(stderr, "model resolution failed: %v\n", err)
				return 1
			}
			return runtimeStartModelWithProfile(model.Path, profile, stdout, stderr)
		}
		modelPath, err := defaultModelPath()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return runtimeStartModelWithProfile(modelPath, profile, stdout, stderr)
	case "stop":
		return runtimeStop(stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown runtime command: %s\n", args[2])
		return 1
	}
}
