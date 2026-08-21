package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type missionDefinition struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Purpose     string   `json:"purpose"`
	PackIDs     []string `json:"pack_ids"`
	Description string   `json:"description"`
}

var missions = []missionDefinition{
	{ID: "developer", Title: "Can this help me develop software?", Purpose: "Map coding, developer workflow, structured output, and technical writing usefulness.", PackIDs: []string{"developer-core", "structured-output", "writing-summarization"}, Description: "Useful before trusting a local model for commits, PR summaries, code explanation, or bounded review."},
	{ID: "ops", Title: "Can this help me investigate systems?", Purpose: "Map Linux, Docker, and Kubernetes investigation capability.", PackIDs: []string{"linux-investigation", "docker-investigation", "kubernetes-investigation"}, Description: "Focuses on evidence-first diagnosis, not autonomous remediation."},
	{ID: "local-first", Title: "What work could move local?", Purpose: "Sample high-frequency bounded tasks that are plausible cloud-model substitutes.", PackIDs: []string{"structured-output", "developer-core", "writing-summarization", "reasoning-analysis"}, Description: "Finds candidate local-first workloads without declaring them qualified."},
}

func runMissions(stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, "MISSION       PACKS   QUESTION")
	for _, mission := range missions {
		fmt.Fprintf(stdout, "%-13s %-7d %s\n", mission.ID, len(mission.PackIDs), mission.Title)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Run one with: localctl mission run <mission> <model> [--profile=default]")
	return 0
}

func findMission(reference string) (missionDefinition, bool) {
	needle := strings.ToLower(reference)
	for _, mission := range missions {
		if strings.EqualFold(mission.ID, reference) || strings.Contains(strings.ToLower(mission.ID), needle) {
			return mission, true
		}
	}
	return missionDefinition{}, false
}

func runMission(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl mission <show|run> ...")
		return 1
	}
	switch args[2] {
	case "show":
		if len(args) < 4 {
			fmt.Fprintln(stderr, "usage: localctl mission show <mission>")
			return 1
		}
		mission, ok := findMission(args[3])
		if !ok {
			fmt.Fprintf(stderr, "unknown mission: %s\n", args[3])
			return 1
		}
		fmt.Fprintf(stdout, "%s — %s\n", mission.ID, mission.Title)
		fmt.Fprintf(stdout, "purpose: %s\n", mission.Purpose)
		fmt.Fprintf(stdout, "packs: %s\n", strings.Join(mission.PackIDs, ", "))
		fmt.Fprintf(stdout, "boundary: %s\n", mission.Description)
		return 0
	case "run":
		if len(args) < 5 {
			fmt.Fprintln(stderr, "usage: localctl mission run <mission> <model> [--profile=default]")
			return 1
		}
		mission, ok := findMission(args[3])
		if !ok {
			fmt.Fprintf(stderr, "unknown mission: %s\n", args[3])
			return 1
		}
		profileRef := "default"
		for _, arg := range args[5:] {
			if strings.HasPrefix(arg, "--profile=") {
				profileRef = strings.TrimPrefix(arg, "--profile=")
			} else {
				fmt.Fprintf(stderr, "unexpected mission argument: %s\n", arg)
				return 1
			}
		}
		session, finishSession := startSession("mission", mission.ID+" "+args[4])
		defer finishSession()
		fmt.Fprintf(stdout, "Mission: %s\nsession: %s\n\n", mission.Title, session.ID)
		for index, packID := range mission.PackIDs {
			pack, err := findPack(packID)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			fmt.Fprintf(stdout, "===== MISSION PACK %d/%d: %s =====\n", index+1, len(mission.PackIDs), pack.ID)
			if code := executePackRun(pack, args[4], profileRef, stdout, stderr); code != 0 {
				return code
			}
			fmt.Fprintln(stdout)
		}
		fmt.Fprintf(stdout, "Mission complete. Review historical shape with: localctl report %s\n", args[4])
		return 0
	default:
		fmt.Fprintf(stderr, "unknown mission command: %s\n", args[2])
		return 1
	}
}

type workTemplate struct {
	Title       string
	Instruction string
}

var workTemplates = map[string]workTemplate{
	"pr-review":         {Title: "Private PR review", Instruction: "Review the supplied diff or PR text. Separate observed changes from inferred risk. Identify the most important correctness concern and one missing test if supported. Do not invent files or behavior. Maximum 220 words."},
	"summarize":         {Title: "Private technical summary", Instruction: "Summarize the supplied material faithfully and concisely. Preserve uncertainty, failures, numbers, and decisions. Do not add facts that are not in the source. Maximum 220 words."},
	"commit":            {Title: "Private commit draft", Instruction: "Draft one concise conventional-style commit subject for the supplied change. Return only the subject and do not invent scope."},
	"linux-triage":      {Title: "Private Linux triage", Instruction: "Analyze the supplied Linux logs or observations. Separate OBSERVED, HYPOTHESIS, and NEXT CHECK. Prefer the smallest discriminating next command. Do not claim a root cause not proven by the input. Maximum 240 words."},
	"docker-triage":     {Title: "Private Docker triage", Instruction: "Analyze the supplied Docker logs, inspect output, or observations. Separate OBSERVED, HYPOTHESIS, and NEXT CHECK. Respect host/container/network/storage boundaries. Maximum 240 words."},
	"kubernetes-triage": {Title: "Private Kubernetes triage", Instruction: "Analyze the supplied Kubernetes events, describe output, logs, or manifests. Separate OBSERVED, HYPOTHESIS, and NEXT CHECK. Distinguish workload, scheduling, service, probe, storage, RBAC, and network evidence. Maximum 260 words."},
}

func runWork(args []string, stdout, stderr io.Writer) int {
	if len(args) < 4 {
		fmt.Fprintln(stderr, "usage: <input> | localctl work <kind> <model> [--profile=default]")
		fmt.Fprintf(stderr, "kinds: %s\n", workKindList())
		return 1
	}
	template, ok := workTemplates[args[2]]
	if !ok {
		fmt.Fprintf(stderr, "unknown work kind: %s\n", args[2])
		fmt.Fprintf(stderr, "kinds: %s\n", workKindList())
		return 1
	}
	profileRef := "default"
	for _, arg := range args[4:] {
		if strings.HasPrefix(arg, "--profile=") {
			profileRef = strings.TrimPrefix(arg, "--profile=")
		} else {
			fmt.Fprintf(stderr, "unexpected work argument: %s\n", arg)
			return 1
		}
	}
	input, err := io.ReadAll(io.LimitReader(os.Stdin, 2*1024*1024+1))
	if err != nil {
		fmt.Fprintf(stderr, "could not read work input: %v\n", err)
		return 1
	}
	if len(input) == 0 {
		fmt.Fprintln(stderr, "work input was empty; pipe or redirect source material into LocalCTL")
		return 1
	}
	if len(input) > 2*1024*1024 {
		fmt.Fprintln(stderr, "work input exceeds the 2 MiB v0.3 safety limit")
		return 1
	}
	model, err := resolveModel(args[3])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	profile, err := resolveProfile(profileRef)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if _, code := ensureRuntimeForModelProfile(model, profile, stdout, stderr); code != 0 {
		return code
	}
	prompt := template.Instruction + "\n\nSOURCE\n" + string(input)
	item := exercise{ID: "work-" + args[2], Title: template.Title, Category: "work", Difficulty: "real-world", Description: "Private, user-supplied real-work evidence. Not part of canonical benchmark aggregation.", Prompt: prompt, Evaluation: evaluationSpec{Kind: evaluationManual}}
	pack := packManifest{ID: "real-work", Version: "v1", Title: "Private real work", Purpose: "Measure transfer from canonical tests to actual user work.", DoesNotProve: "Canonical benchmark performance."}
	session, finishSession := startSession("work", args[2]+" "+model.ID)
	defer finishSession()
	experiment, finishExperiment := startScopedExperiment("work", args[2], model, profile, pack, "private")
	defer finishExperiment()
	fmt.Fprintf(stdout, "Private real-work run\nkind: %s\nmodel: %s\nprofile: %s\nsession: %s\nexperiment: %s\n\n", args[2], model.Name, profile.ID, session.ID, experiment.ID)
	record, outcome, runErr := runObservedItem(item, model, profile)
	if runErr != nil {
		fmt.Fprintf(stderr, "inference failed: %v\n", runErr)
		if record.RunID != "" {
			fmt.Fprintf(stderr, "saved: %s\n", record.RunID)
		}
		return 1
	}
	fmt.Fprintln(stdout, outcome.Content)
	fmt.Fprintf(stdout, "\nrun: %s\n", record.RunID)
	fmt.Fprintln(stdout, "input class: private — excluded from canonical pack evidence sharing by design")
	fmt.Fprintf(stdout, "Judge it later: localctl judge %s good|partial|bad \"reason\"\n", record.RunID)
	return 0
}

func workKindList() string {
	var kinds []string
	for kind := range workTemplates {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return strings.Join(kinds, ", ")
}
