package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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

	healthy := indexErr == nil && missingPrompt == 0 && missingResponse == 0 && missingFromIndex == 0 && duplicateIndexEntries == 0
	fmt.Fprintln(stdout)
	if healthy {
		fmt.Fprintln(stdout, "history status: COMPLETE")
		fmt.Fprintln(stdout, "Every discovered observation has its required files and an index entry.")
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
	return 1
}
