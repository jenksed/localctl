package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func runEvidence(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl evidence <audit|rebuild-index|path>")
		return 1
	}

	switch args[2] {
	case "audit":
		return runEvidenceAudit(stdout, stderr)
	case "rebuild-index":
		count, err := rebuildObservationIndex()
		if err != nil {
			fmt.Fprintf(stderr, "could not rebuild evidence index: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "rebuilt evidence index from %d observation files\n", count)
		return 0
	case "path":
		root, err := evidenceRoot()
		if err != nil {
			fmt.Fprintf(stderr, "could not determine evidence path: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, root)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown evidence command: %s\n", args[2])
		return 1
	}
}

func runEvidenceAudit(stdout, stderr io.Writer) int {
	records, paths, err := scanObservationFiles()
	if err != nil {
		fmt.Fprintf(stderr, "could not scan observation files: %v\n", err)
		return 1
	}

	indexed, indexErr := listObservations()
	indexIDs := map[string]int{}
	if indexErr == nil {
		for _, record := range indexed {
			indexIDs[record.RunID]++
		}
	}

	schemaCounts := map[int]int{}
	models := map[string]int{}
	missingPrompt := 0
	missingResponse := 0
	missingFromIndex := 0
	duplicateIndexEntries := 0
	judgments := 0
	incompleteV3 := 0
	v3MissingFields := map[string]int{}

	for _, count := range indexIDs {
		if count > 1 {
			duplicateIndexEntries += count - 1
		}
	}

	for _, record := range records {
		schemaCounts[record.SchemaVersion]++
		models[record.Model.Name]++
		runDir := paths[record.RunID]
		if _, err := os.Stat(filepath.Join(runDir, "prompt.txt")); err != nil {
			missingPrompt++
		}
		if record.Result.Status == "succeeded" {
			if _, err := os.Stat(filepath.Join(runDir, "response.txt")); err != nil {
				missingResponse++
			}
		}
		if _, err := os.Stat(filepath.Join(runDir, "judgment.json")); err == nil {
			judgments++
		}
		if indexIDs[record.RunID] == 0 {
			missingFromIndex++
		}
		if record.SchemaVersion >= 3 {
			missing := v3ObservationMissing(record)
			if len(missing) > 0 {
				incompleteV3++
				for _, field := range missing {
					v3MissingFields[field]++
				}
			}
		}
	}

	fmt.Fprintln(stdout, "LocalCTL evidence audit")
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "observation files:      %d\n", len(records))
	if indexErr != nil {
		fmt.Fprintf(stdout, "index:                  unreadable (%v)\n", indexErr)
	} else {
		fmt.Fprintf(stdout, "index entries:          %d\n", len(indexed))
		fmt.Fprintf(stdout, "unindexed observations: %d\n", missingFromIndex)
		fmt.Fprintf(stdout, "duplicate index rows:   %d\n", duplicateIndexEntries)
	}
	fmt.Fprintf(stdout, "missing prompts:        %d\n", missingPrompt)
	fmt.Fprintf(stdout, "missing responses:      %d\n", missingResponse)
	fmt.Fprintf(stdout, "human judgments:        %d\n", judgments)
	fmt.Fprintf(stdout, "incomplete v3 records:  %d\n", incompleteV3)

	if len(v3MissingFields) > 0 {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Missing v3 provenance fields")
		var names []string
		for name := range v3MissingFields {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Fprintf(stdout, "%-30s %d\n", name, v3MissingFields[name])
		}
	}

	var schemaVersions []int
	for version := range schemaCounts {
		schemaVersions = append(schemaVersions, version)
	}
	sort.Ints(schemaVersions)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Schema history")
	if len(schemaVersions) == 0 {
		fmt.Fprintln(stdout, "no saved runs")
	}
	for _, version := range schemaVersions {
		fmt.Fprintf(stdout, "v%d: %d runs\n", version, schemaCounts[version])
	}

	var modelNames []string
	for name := range models {
		modelNames = append(modelNames, name)
	}
	sort.Strings(modelNames)
	if len(modelNames) > 0 {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Runs by model")
		for _, name := range modelNames {
			fmt.Fprintf(stdout, "%-36s %d\n", name, models[name])
		}
	}

	healthy := indexErr == nil && missingPrompt == 0 && missingResponse == 0 && missingFromIndex == 0 && duplicateIndexEntries == 0 && incompleteV3 == 0
	fmt.Fprintln(stdout)
	if healthy {
		fmt.Fprintln(stdout, "history status: COMPLETE")
		fmt.Fprintln(stdout, "Every discovered observation has its required files and index coverage; v3 runs also satisfy the v0.3 provenance contract.")
		fmt.Fprintln(stdout, "Older schema versions remain historical evidence; LocalCTL does not rewrite them.")
		return 0
	}

	fmt.Fprintln(stdout, "history status: NEEDS ATTENTION")
	if indexErr != nil || missingFromIndex > 0 || duplicateIndexEntries > 0 {
		fmt.Fprintln(stdout, "Index problems can be repaired from immutable observation files with:")
		fmt.Fprintln(stdout, "  localctl evidence rebuild-index")
	}
	if missingPrompt > 0 || missingResponse > 0 {
		fmt.Fprintln(stdout, "Missing prompt/response artifacts cannot be reconstructed honestly from the index alone.")
	}
	if incompleteV3 > 0 {
		fmt.Fprintln(stdout, "Incomplete v3 provenance is reported rather than retroactively invented. Re-run the affected workload if stronger current evidence is needed.")
	}
	return 1
}

func v3ObservationMissing(record runObservation) []string {
	var missing []string
	add := func(name string, condition bool) {
		if condition {
			missing = append(missing, name)
		}
	}
	add("experiment.id", record.Experiment.ID == "")
	add("exercise.version", record.Exercise.Version == "")
	add("exercise.prompt_sha256", record.Exercise.PromptSHA256 == "")
	add("pack.id", record.Pack.ID == "")
	add("pack.version", record.Pack.Version == "")
	add("input.class", record.Input.Class != "canonical" && record.Input.Class != "private")
	add("machine.os", record.Machine.OS == "")
	add("machine.architecture", record.Machine.Architecture == "")
	add("localctl.version", record.LocalCTL.Version == "")
	add("model.sha256", record.Model.SHA256 == "")
	add("runtime.kind", record.Runtime.Kind == "")
	add("runtime.url", record.Runtime.URL == "")
	add("profile.id", record.Profile.ID == "")
	add("configuration.context", record.Configuration.Context <= 0)
	add("configuration.max_tokens", record.Configuration.MaxTokens <= 0)
	add("validation.authority", record.Validation.Authority == "")
	if record.Result.Status == "succeeded" {
		add("result.response_sha256", record.Result.ResponseSHA256 == "")
	}
	return missing
}

func joinMissingFields(fields []string) string {
	return strings.Join(fields, ", ")
}
