package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

type modelRadarCandidate struct {
	Name       string `json:"name"`
	Repository string `json:"repository,omitempty"`
	Quant      string `json:"quant,omitempty"`
	FileSize   string `json:"file_size,omitempty"`
	Tier       string `json:"tier,omitempty"`
	Why        string `json:"why,omitempty"`
	Source     string `json:"source,omitempty"`
}

var m1ProRadar = []modelRadarCandidate{
	{
		Name:       "Qwen3.5-9B",
		Repository: "openresearchtools/Qwen3.5-9B-GGUF",
		Quant:      "Q4_K_M",
		FileSize:   "5.63 GB",
		Tier:       "priority",
		Why:        "modern 9B candidate in the same broad footprint class as the current 8B/9B set",
		Source:     "https://huggingface.co/openresearchtools/Qwen3.5-9B-GGUF",
	},
	{
		Name:       "Gemma 4 E4B IT",
		Repository: "ggml-org/gemma-4-E4B-it-GGUF",
		Quant:      "Q4_0",
		FileSize:   "4.59 GB",
		Tier:       "priority",
		Why:        "current 8B-class family contrast with a relatively compact published GGUF",
		Source:     "https://huggingface.co/ggml-org/gemma-4-E4B-it-GGUF",
	},
	{
		Name:       "Qwen3.5-4B Instruct",
		Repository: "openresearchtools/Qwen3.5-4B-Instruct-GGUF",
		Quant:      "Q4_K_M",
		FileSize:   "2.71 GB",
		Tier:       "speed-control",
		Why:        "small control for finding the lower edge of useful coding and ops work",
		Source:     "https://huggingface.co/openresearchtools/Qwen3.5-4B-Instruct-GGUF",
	},
	{
		Name:       "Gemma 4 12B IT",
		Repository: "ggml-org/gemma-4-12B-it-GGUF",
		Quant:      "Q4_0",
		FileSize:   "7.22 GB",
		Tier:       "stretch",
		Why:        "larger candidate for testing whether extra quality justifies tighter memory headroom",
		Source:     "https://huggingface.co/ggml-org/gemma-4-12B-it-GGUF",
	},
	{
		Name:       "Qwen3 14B",
		Repository: "Qwen/Qwen3-14B-GGUF",
		Quant:      "Q4_K_M",
		FileSize:   "9.00 GB",
		Tier:       "stretch",
		Why:        "higher-capacity control for quality-versus-speed and memory tradeoff experiments",
		Source:     "https://huggingface.co/Qwen/Qwen3-14B-GGUF",
	},
}

type hfModelResult struct {
	ID           string   `json:"id"`
	LastModified string   `json:"lastModified"`
	Downloads    int      `json:"downloads"`
	Tags         []string `json:"tags"`
}

var plausibleLocalSize = regexp.MustCompile(`(?i)(?:^|[-_/])(e?[3-9]|1[0-4])b(?:[-_/]|$)`)

func runExplore(args []string, stdout, stderr io.Writer) int {
	if len(args) >= 3 {
		switch args[2] {
		case "select":
			return runExploreSelect(args, stdout, stderr)
		case "unselect":
			return runExploreUnselect(args, stdout, stderr)
		case "audit", "selected":
			return runExploreAudit(stdout, stderr)
		case "--live":
			showAll := len(args) >= 4 && args[3] == "--all"
			if len(args) >= 4 && !showAll {
				fmt.Fprintf(stderr, "unexpected explore argument: %s\n", args[3])
				return 1
			}
			return runLiveModelRadar(showAll, stdout, stderr)
		case "help", "--help", "-h":
			printExploreHelp(stdout)
			return 0
		default:
			fmt.Fprintf(stderr, "unknown explore command: %s\n", args[2])
			fmt.Fprintln(stderr, "Run 'localctl explore help' for usage.")
			return 1
		}
	}
	return runCuratedModelRadar(stdout, stderr)
}

func printExploreHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "LocalCTL model exploration")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "  localctl explore                         show new curated candidates")
	fmt.Fprintln(stdout, "  localctl explore select <candidate>      move a candidate into the audit queue")
	fmt.Fprintln(stdout, "  localctl explore audit                   inspect the audit queue")
	fmt.Fprintln(stdout, "  localctl explore unselect <candidate>    return a candidate to discovery")
	fmt.Fprintln(stdout, "  localctl explore --live                  show 5 recent Hugging Face candidates")
	fmt.Fprintln(stdout, "  localctl explore --live --all            show up to 15 recent candidates")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Installed GGUFs are hidden automatically; once installed, use 'localctl models' and 'localctl audition <model>'.")
}

func loadExploreContext() ([]modelArtifact, exploreState, error) {
	models, err := discoverModels()
	if err != nil {
		return nil, exploreState{}, err
	}
	state, err := loadExploreState()
	if err != nil {
		return nil, exploreState{}, err
	}
	state, changed := pruneInstalledSelections(state, models)
	if changed {
		if err := saveExploreState(state); err != nil {
			return nil, exploreState{}, err
		}
	}
	return models, state, nil
}

func runCuratedModelRadar(stdout, stderr io.Writer) int {
	models, state, err := loadExploreContext()
	if err != nil {
		fmt.Fprintf(stderr, "could not load model exploration state: %v\n", err)
		return 1
	}
	selected := selectedCandidateIDs(state)
	var available []modelRadarCandidate
	installedHidden := 0
	for _, candidate := range m1ProRadar {
		if candidateInstalled(candidate, models) {
			installedHidden++
			continue
		}
		if selected[candidateKey(candidate)] {
			continue
		}
		available = append(available, candidate)
	}

	fmt.Fprintln(stdout, "LocalCTL model radar")
	fmt.Fprintf(stdout, "%d new · %d selected for audit · %d installed models already in LocalCTL\n\n", len(available), len(state.Selected), len(models))
	if len(available) == 0 {
		fmt.Fprintln(stdout, "No new curated candidates right now.")
	} else {
		fmt.Fprintln(stdout, "CANDIDATE                 TIER           QUANT       FILE       WHY")
		for _, candidate := range available {
			fmt.Fprintf(stdout, "%-25s %-14s %-11s %-10s %s\n", candidate.Name, candidate.Tier, candidate.Quant, candidate.FileSize, candidate.Why)
		}
	}
	if installedHidden > 0 {
		fmt.Fprintf(stdout, "\n%d curated candidate(s) already installed were hidden from discovery.\n", installedHidden)
	}
	if len(state.Selected) > 0 {
		var names []string
		for _, item := range state.Selected {
			names = append(names, item.Candidate.Name)
		}
		fmt.Fprintf(stdout, "Audit queue: %s\n", strings.Join(names, ", "))
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Select one:  localctl explore select <candidate>")
	fmt.Fprintln(stdout, "Audit queue: localctl explore audit")
	fmt.Fprintln(stdout, "More radar:  localctl explore --live")
	return 0
}

func resolveExploreCandidate(reference string) (modelRadarCandidate, error) {
	needle := strings.ToLower(strings.TrimSpace(reference))
	var matches []modelRadarCandidate
	for _, candidate := range m1ProRadar {
		haystack := strings.ToLower(candidate.Name + " " + candidate.Repository + " " + candidateKey(candidate))
		if strings.EqualFold(candidate.Name, reference) || strings.EqualFold(candidate.Repository, reference) || candidateKey(candidate) == needle {
			return candidate, nil
		}
		if strings.Contains(haystack, needle) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		var names []string
		for _, candidate := range matches {
			names = append(names, candidate.Name)
		}
		return modelRadarCandidate{}, fmt.Errorf("candidate reference %q is ambiguous: %s", reference, strings.Join(names, ", "))
	}
	if strings.Contains(reference, "/") {
		parts := strings.Split(strings.Trim(reference, "/"), "/")
		name := parts[len(parts)-1]
		return modelRadarCandidate{Name: name, Repository: strings.Trim(reference, "/"), Tier: "live", Why: "user-selected live discovery candidate", Source: "https://huggingface.co/" + strings.Trim(reference, "/")}, nil
	}
	return modelRadarCandidate{}, fmt.Errorf("candidate %q not found; use a curated name or full Hugging Face repository such as owner/model-GGUF", reference)
}

func runExploreSelect(args []string, stdout, stderr io.Writer) int {
	if len(args) < 4 {
		fmt.Fprintln(stderr, "usage: localctl explore select <candidate-or-huggingface-repository>")
		return 1
	}
	candidate, err := resolveExploreCandidate(strings.Join(args[3:], " "))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	models, state, err := loadExploreContext()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if candidateInstalled(candidate, models) {
		fmt.Fprintf(stdout, "%s is already installed; use 'localctl models' and 'localctl audition <model>' instead.\n", candidate.Name)
		return 0
	}
	before := len(state.Selected)
	state = selectExploreCandidate(state, candidate)
	if err := saveExploreState(state); err != nil {
		fmt.Fprintf(stderr, "could not save audit queue: %v\n", err)
		return 1
	}
	if len(state.Selected) == before {
		fmt.Fprintf(stdout, "%s is already selected for audit.\n", candidate.Name)
	} else {
		fmt.Fprintf(stdout, "Selected for audit: %s\n", candidate.Name)
	}
	fmt.Fprintln(stdout, "Next: localctl explore audit")
	return 0
}

func runExploreUnselect(args []string, stdout, stderr io.Writer) int {
	if len(args) < 4 {
		fmt.Fprintln(stderr, "usage: localctl explore unselect <candidate>")
		return 1
	}
	_, state, err := loadExploreContext()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	state, removed := unselectExploreCandidate(state, strings.Join(args[3:], " "))
	if !removed {
		fmt.Fprintln(stderr, "candidate is not in the audit queue")
		return 1
	}
	if err := saveExploreState(state); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "Candidate returned to discovery.")
	return 0
}

func runExploreAudit(stdout, stderr io.Writer) int {
	_, state, err := loadExploreContext()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "LocalCTL candidate audit queue")
	fmt.Fprintln(stdout)
	if len(state.Selected) == 0 {
		fmt.Fprintln(stdout, "No candidates selected.")
		fmt.Fprintln(stdout, "Choose one with: localctl explore select <candidate>")
		return 0
	}
	for index, item := range state.Selected {
		candidate := item.Candidate
		fmt.Fprintf(stdout, "%d. %s", index+1, candidate.Name)
		if candidate.Tier != "" {
			fmt.Fprintf(stdout, "  [%s]", candidate.Tier)
		}
		fmt.Fprintln(stdout)
		if candidate.Quant != "" || candidate.FileSize != "" {
			fmt.Fprintf(stdout, "   target: %s  %s\n", emptyAsDash(candidate.Quant), emptyAsDash(candidate.FileSize))
		}
		if candidate.Why != "" {
			fmt.Fprintf(stdout, "   why:    %s\n", candidate.Why)
		}
		if candidate.Source != "" {
			fmt.Fprintf(stdout, "   source: %s\n", candidate.Source)
		}
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Install candidates with your preferred model manager. Once LocalCTL discovers the GGUF, it is removed from this queue automatically.")
	fmt.Fprintln(stdout, "Then use: localctl models → localctl audition <model>")
	return 0
}

func emptyAsDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

func runLiveModelRadar(showAll bool, stdout, stderr io.Writer) int {
	const endpoint = "https://huggingface.co/api/models?search=GGUF&sort=lastModified&direction=-1&limit=100&full=true"
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		fmt.Fprintf(stderr, "could not create Hugging Face request: %v\n", err)
		return 1
	}
	req.Header.Set("User-Agent", "localctl-model-radar/1")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "live model discovery failed: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintf(stderr, "live model discovery failed: HTTP %s\n", resp.Status)
		return 1
	}
	var results []hfModelResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		fmt.Fprintf(stderr, "could not decode Hugging Face results: %v\n", err)
		return 1
	}

	models, state, err := loadExploreContext()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	selected := selectedCandidateIDs(state)
	var candidates []hfModelResult
	for _, result := range results {
		lower := strings.ToLower(result.ID)
		if !strings.Contains(lower, "gguf") || !plausibleLocalSize.MatchString(lower) {
			continue
		}
		if strings.Contains(lower, "embedding") || strings.Contains(lower, "reranker") || strings.Contains(lower, "tts") {
			continue
		}
		parts := strings.Split(result.ID, "/")
		name := parts[len(parts)-1]
		candidate := modelRadarCandidate{Name: name, Repository: result.ID}
		if candidateInstalled(candidate, models) || selected[candidateKey(candidate)] {
			continue
		}
		candidates = append(candidates, result)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].LastModified > candidates[j].LastModified
	})
	limit := 5
	if showAll {
		limit = 15
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	fmt.Fprintln(stdout, "Live Hugging Face GGUF radar")
	fmt.Fprintln(stdout, "Discovery only — recent repositories whose names suggest roughly 3B–14B scale. Installed and already-selected candidates are hidden.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "UPDATED               DOWNLOADS   REPOSITORY")
	for _, candidate := range candidates {
		updated := candidate.LastModified
		if len(updated) > 20 {
			updated = updated[:20]
		}
		fmt.Fprintf(stdout, "%-21s %-11d %s\n", updated, candidate.Downloads, candidate.ID)
	}
	if len(candidates) == 0 {
		fmt.Fprintln(stdout, "no new candidates matched the current conservative filter")
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Select one by repository: localctl explore select <owner/model-GGUF>")
	if !showAll {
		fmt.Fprintln(stdout, "Need more? localctl explore --live --all")
	}
	return 0
}
