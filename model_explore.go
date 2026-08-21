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
	Name      string
	Quant     string
	FileSize  string
	Tier      string
	Why       string
	Source    string
}

var m1ProRadar = []modelRadarCandidate{
	{
		Name:     "Qwen3.5-9B",
		Quant:    "Q4_K_M",
		FileSize: "5.63 GB",
		Tier:     "priority",
		Why:      "modern 9B candidate in the same broad footprint class as the current 8B/9B set; high-value developer + general capability comparison",
		Source:   "https://huggingface.co/openresearchtools/Qwen3.5-9B-GGUF",
	},
	{
		Name:     "Gemma 4 E4B IT",
		Quant:    "Q4_0",
		FileSize: "4.59 GB",
		Tier:     "priority",
		Why:      "current Gemma 4 8B-class candidate with a smaller GGUF file than the existing local set; useful family/architecture contrast",
		Source:   "https://huggingface.co/ggml-org/gemma-4-E4B-it-GGUF",
	},
	{
		Name:     "Qwen3.5-4B Instruct",
		Quant:    "Q4_K_M",
		FileSize: "2.71 GB",
		Tier:     "speed-control",
		Why:      "small modern instruct candidate for finding the lower edge of useful coding/ops work at much lower memory and likely latency cost",
		Source:   "https://huggingface.co/openresearchtools/Qwen3.5-4B-Instruct-GGUF",
	},
	{
		Name:     "Gemma 4 12B IT",
		Quant:    "Q4_0",
		FileSize: "7.22 GB",
		Tier:     "stretch",
		Why:      "larger current candidate worth testing cautiously on 16 GB unified memory; model file fits comfortably but context/KV cache/runtime headroom still decides practical fit",
		Source:   "https://huggingface.co/ggml-org/gemma-4-12B-it-GGUF",
	},
	{
		Name:     "Qwen3 14B",
		Quant:    "Q4_K_M",
		FileSize: "9.00 GB",
		Tier:     "stretch",
		Why:      "higher-capacity control for measuring whether extra quality offsets slower generation and tighter unified-memory headroom",
		Source:   "https://huggingface.co/Qwen/Qwen3-14B-GGUF",
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
	if len(args) >= 3 && args[2] == "--live" {
		return runLiveModelRadar(stdout, stderr)
	}

	fmt.Fprintln(stdout, "LocalCTL model radar — M1 Pro / 16 GB")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Curated candidates for investigation, not compatibility or quality guarantees.")
	fmt.Fprintln(stdout, "Published GGUF file size is not total memory use; context/KV cache and runtime overhead still need headroom.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "TIER           MODEL                    QUANT       FILE       WHY")
	for _, candidate := range m1ProRadar {
		fmt.Fprintf(stdout, "%-14s %-24s %-11s %-10s %s\n", candidate.Tier, candidate.Name, candidate.Quant, candidate.FileSize, candidate.Why)
		fmt.Fprintf(stdout, "               source: %s\n", candidate.Source)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Recommended next additions: Qwen3.5-9B first, then Gemma 4 E4B IT; use Qwen3.5-4B as the speed/control model.")
	fmt.Fprintln(stdout, "Treat the 12B/14B entries as stretch experiments, not assumed good fits.")
	fmt.Fprintln(stdout, "Use 'localctl explore --live' to surface recently updated GGUF repositories in the 3B–14B naming range.")
	return 0
}

func runLiveModelRadar(stdout, stderr io.Writer) int {
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

	var candidates []hfModelResult
	for _, result := range results {
		lower := strings.ToLower(result.ID)
		if !strings.Contains(lower, "gguf") || !plausibleLocalSize.MatchString(lower) {
			continue
		}
		if strings.Contains(lower, "embedding") || strings.Contains(lower, "reranker") || strings.Contains(lower, "tts") {
			continue
		}
		candidates = append(candidates, result)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].LastModified > candidates[j].LastModified
	})
	if len(candidates) > 15 {
		candidates = candidates[:15]
	}

	fmt.Fprintln(stdout, "Live Hugging Face GGUF radar")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Recently updated repositories with names suggesting roughly 3B–14B scale.")
	fmt.Fprintln(stdout, "Discovery only: naming does not prove text capability, quant size, llama.cpp compatibility, model quality, or fit on this machine.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "UPDATED               DOWNLOADS   REPOSITORY")
	for _, candidate := range candidates {
		updated := candidate.LastModified
		if len(updated) > 20 {
			updated = updated[:20]
		}
		fmt.Fprintf(stdout, "%-21s %-11d %s\n", updated, candidate.Downloads, candidate.ID)
		fmt.Fprintf(stdout, "                                  https://huggingface.co/%s\n", candidate.ID)
	}
	if len(candidates) == 0 {
		fmt.Fprintln(stdout, "no candidates matched the current conservative filter")
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Use the curated 'localctl explore' list when you want candidates already selected for this machine class.")
	return 0
}
