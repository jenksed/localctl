package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const radarCriteriaSchemaVersion = 1

type radarCriteria struct {
	SchemaVersion int      `json:"schema_version"`
	Fit           string   `json:"fit"`
	Uses          []string `json:"uses"`
	MaxParamsB    float64  `json:"max_params_b,omitempty"`
	ReleaseDays   int      `json:"release_days,omitempty"`
	MinDownloads  int      `json:"min_downloads,omitempty"`
	License       string   `json:"license"`
}

type radarCandidate struct {
	Repository      string   `json:"repository"`
	ParametersB     float64  `json:"parameter_hint_b,omitempty"`
	LastModified    string   `json:"last_modified,omitempty"`
	Downloads       int      `json:"downloads,omitempty"`
	UseMatches      []string `json:"use_matches,omitempty"`
	License         string   `json:"license,omitempty"`
	Fit             string   `json:"fit_screen"`
	LocalStatus     string   `json:"local_status"`
	Why             string   `json:"why"`
	Source          string   `json:"source"`
	OpportunityRank int      `json:"opportunity_rank"`
}

var parameterCountPattern = regexp.MustCompile(`(?i)(?:^|[-_/.])e?(\d+(?:\.\d+)?)b(?:[-_/.]|$)`)

func defaultRadarCriteria() radarCriteria {
	return radarCriteria{
		SchemaVersion: radarCriteriaSchemaVersion,
		Fit:           "comfortable",
		Uses:          []string{"coding", "debugging", "review"},
		ReleaseDays:   90,
		License:       "any",
	}
}

func radarCriteriaPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".localctl", "radar.json"), nil
}

func loadRadarCriteria() (radarCriteria, error) {
	path, err := radarCriteriaPath()
	if err != nil {
		return radarCriteria{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultRadarCriteria(), nil
		}
		return radarCriteria{}, err
	}
	criteria := defaultRadarCriteria()
	if err := json.Unmarshal(data, &criteria); err != nil {
		return radarCriteria{}, fmt.Errorf("decode radar criteria: %w", err)
	}
	if criteria.SchemaVersion == 0 {
		criteria.SchemaVersion = radarCriteriaSchemaVersion
	}
	if err := validateRadarCriteria(criteria); err != nil {
		return radarCriteria{}, err
	}
	criteria.Uses = normalizeRadarUses(criteria.Uses)
	return criteria, nil
}

func saveRadarCriteria(criteria radarCriteria) error {
	if err := validateRadarCriteria(criteria); err != nil {
		return err
	}
	path, err := radarCriteriaPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	criteria.SchemaVersion = radarCriteriaSchemaVersion
	criteria.Uses = normalizeRadarUses(criteria.Uses)
	data, err := json.MarshalIndent(criteria, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func resetRadarCriteria() error {
	path, err := radarCriteriaPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func validateRadarCriteria(criteria radarCriteria) error {
	switch criteria.Fit {
	case "comfortable", "stretch", "any":
	default:
		return fmt.Errorf("radar fit must be comfortable, stretch, or any")
	}
	if criteria.MaxParamsB < 0 {
		return fmt.Errorf("max_params_b cannot be negative")
	}
	if criteria.ReleaseDays < 0 || criteria.MinDownloads < 0 {
		return fmt.Errorf("release_days and min_downloads cannot be negative")
	}
	switch criteria.License {
	case "any", "known-commercial", "permissive":
	default:
		return fmt.Errorf("radar license must be any, known-commercial, or permissive")
	}
	validUses := map[string]bool{
		"coding": true, "debugging": true, "review": true, "general": true,
		"reasoning": true, "tool-use": true, "embedding": true, "reranking": true,
	}
	for _, use := range normalizeRadarUses(criteria.Uses) {
		if !validUses[use] {
			return fmt.Errorf("unsupported radar use %q", use)
		}
	}
	return nil
}

func normalizeRadarUses(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.ToLower(strings.TrimSpace(part))
			if part == "" || seen[part] {
				continue
			}
			seen[part] = true
			out = append(out, part)
		}
	}
	sort.Strings(out)
	return out
}

func machineMemoryGB(machine machineFingerprintRecord) float64 {
	if machine.MemoryBytes <= 0 {
		return 0
	}
	return float64(machine.MemoryBytes) / float64(int64(1)<<30)
}

// automaticRadarMaxParams produces a discovery threshold, not a runtime-fit claim.
// It reserves meaningful memory for the OS, editor, KV cache, and runtime overhead,
// then uses a deliberately conservative Q4-class bytes-per-parameter estimate.
// Hugging Face's parameter metadata enforces this threshold for live discovery.
func automaticRadarMaxParams(machine machineFingerprintRecord, fit string) float64 {
	memoryGB := machineMemoryGB(machine)
	if memoryGB <= 0 || fit == "any" {
		return 0
	}
	fraction := 0.55
	if fit == "stretch" {
		fraction = 0.72
	}
	budgetGB := memoryGB * fraction
	const q4ClassGBPerBillionParams = 0.65
	return math.Floor((budgetGB/q4ClassGBPerBillionParams)*10) / 10
}

func effectiveRadarMaxParams(criteria radarCriteria, machine machineFingerprintRecord) float64 {
	if criteria.MaxParamsB > 0 {
		return criteria.MaxParamsB
	}
	return automaticRadarMaxParams(machine, criteria.Fit)
}

// parametersFromModelID is deliberately only a display hint. Repository naming is
// not authoritative enough to enforce machine fit; the live query uses Hugging
// Face's num_parameters filter for that boundary.
func parametersFromModelID(id string) float64 {
	matches := parameterCountPattern.FindAllStringSubmatch(id, -1)
	var result float64
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		value, err := strconv.ParseFloat(match[1], 64)
		if err == nil && value > result {
			result = value
		}
	}
	return result
}

func hfLicense(tags []string) string {
	for _, tag := range tags {
		lower := strings.ToLower(tag)
		if strings.HasPrefix(lower, "license:") {
			return strings.TrimPrefix(lower, "license:")
		}
	}
	return ""
}

func licensePassesFilter(license, filter string) bool {
	if filter == "any" {
		return true
	}
	permissive := map[string]bool{
		"apache-2.0": true, "mit": true, "bsd": true, "bsd-2-clause": true,
		"bsd-3-clause": true, "isc": true,
	}
	if permissive[license] {
		return true
	}
	if filter == "permissive" {
		return false
	}
	knownCommercial := map[string]bool{
		"cc-by-4.0": true, "cc-by-sa-4.0": true,
	}
	return knownCommercial[license]
}

func radarUseMatches(result hfModelResult, requested []string) []string {
	requestedSet := map[string]bool{}
	for _, use := range normalizeRadarUses(requested) {
		requestedSet[use] = true
	}
	corpus := strings.ToLower(result.ID + " " + strings.Join(result.Tags, " "))
	candidate := map[string]bool{
		"coding":    strings.Contains(corpus, "code") || strings.Contains(corpus, "coder") || strings.Contains(corpus, "program"),
		"debugging": strings.Contains(corpus, "code") || strings.Contains(corpus, "coder") || strings.Contains(corpus, "debug"),
		"review":    strings.Contains(corpus, "code") || strings.Contains(corpus, "coder") || strings.Contains(corpus, "review"),
		"general":   strings.Contains(corpus, "instruct") || strings.Contains(corpus, "chat") || strings.Contains(corpus, "text-generation"),
		"reasoning": strings.Contains(corpus, "reason") || strings.Contains(corpus, "thinking"),
		"tool-use":  strings.Contains(corpus, "tool") || strings.Contains(corpus, "function"),
		"embedding": strings.Contains(corpus, "embedding") || strings.Contains(corpus, "feature-extraction"),
		"reranking": strings.Contains(corpus, "rerank") || strings.Contains(corpus, "reranker"),
	}
	var matches []string
	for use := range requestedSet {
		if candidate[use] {
			matches = append(matches, use)
		}
	}
	sort.Strings(matches)
	return matches
}

func requestedSpecialist(criteria radarCriteria, kind string) bool {
	for _, use := range criteria.Uses {
		if use == kind {
			return true
		}
	}
	return false
}

func isSpecialistResult(result hfModelResult, kind string) bool {
	corpus := strings.ToLower(result.ID + " " + strings.Join(result.Tags, " "))
	switch kind {
	case "embedding":
		return strings.Contains(corpus, "embedding") || strings.Contains(corpus, "feature-extraction")
	case "reranking":
		return strings.Contains(corpus, "rerank") || strings.Contains(corpus, "reranker")
	default:
		return false
	}
}

func fitScreenLabel(criteria radarCriteria, maxParams float64) string {
	if maxParams <= 0 {
		return "ANY"
	}
	return strings.ToUpper(criteria.Fit) + "_SCREEN"
}

// rankRadarResults intentionally does not enforce parameter count from repository
// names. Live results have already passed Hugging Face's num_parameters screen.
// This function ranks metadata candidates after that external screening step.
func rankRadarResults(results []hfModelResult, criteria radarCriteria, machine machineFingerprintRecord, installed []modelArtifact, now time.Time) []radarCandidate {
	maxParams := effectiveRadarMaxParams(criteria, machine)
	var candidates []radarCandidate
	for _, result := range results {
		if !hasHFGGUFSignal(result) {
			continue
		}
		embedding := isSpecialistResult(result, "embedding")
		reranking := isSpecialistResult(result, "reranking")
		if embedding && !requestedSpecialist(criteria, "embedding") {
			continue
		}
		if reranking && !requestedSpecialist(criteria, "reranking") {
			continue
		}
		modified, _ := time.Parse(time.RFC3339, result.LastModified)
		if criteria.ReleaseDays > 0 {
			if modified.IsZero() || now.Sub(modified) > time.Duration(criteria.ReleaseDays)*24*time.Hour {
				continue
			}
		}
		if result.Downloads < criteria.MinDownloads {
			continue
		}
		license := hfLicense(result.Tags)
		if !licensePassesFilter(license, criteria.License) {
			continue
		}
		parts := strings.Split(result.ID, "/")
		name := parts[len(parts)-1]
		installedCandidate := modelRadarCandidate{Name: name, Repository: result.ID}
		if candidateInstalled(installedCandidate, installed) {
			continue
		}
		matches := radarUseMatches(result, criteria.Uses)
		recencyRank := 0
		if !modified.IsZero() {
			days := int(now.Sub(modified).Hours() / 24)
			switch {
			case days <= 7:
				recencyRank = 3
			case days <= 30:
				recencyRank = 2
			case days <= 90:
				recencyRank = 1
			}
		}
		rank := len(matches)*100 + recencyRank*10
		why := "new GGUF candidate that passed your current Hugging Face discovery screen"
		if len(matches) > 0 {
			why = "matches requested uses: " + strings.Join(matches, ", ")
		}
		candidates = append(candidates, radarCandidate{
			Repository:      result.ID,
			ParametersB:     parametersFromModelID(result.ID),
			LastModified:    result.LastModified,
			Downloads:       result.Downloads,
			UseMatches:      matches,
			License:         license,
			Fit:             fitScreenLabel(criteria, maxParams),
			LocalStatus:     "UNTESTED_LOCAL",
			Why:             why,
			Source:          "https://huggingface.co/" + result.ID,
			OpportunityRank: rank,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].OpportunityRank != candidates[j].OpportunityRank {
			return candidates[i].OpportunityRank > candidates[j].OpportunityRank
		}
		a, _ := time.Parse(time.RFC3339, candidates[i].LastModified)
		b, _ := time.Parse(time.RFC3339, candidates[j].LastModified)
		if !a.Equal(b) {
			return a.After(b)
		}
		if candidates[i].Downloads != candidates[j].Downloads {
			return candidates[i].Downloads > candidates[j].Downloads
		}
		return candidates[i].Repository < candidates[j].Repository
	})
	return candidates
}

func hasHFGGUFSignal(result hfModelResult) bool {
	if strings.Contains(strings.ToLower(result.ID), "gguf") {
		return true
	}
	for _, tag := range result.Tags {
		if strings.EqualFold(tag, "gguf") {
			return true
		}
	}
	return false
}

func radarQueryURL(criteria radarCriteria, machine machineFingerprintRecord) string {
	values := url.Values{}
	values.Set("filter", "gguf")
	values.Set("sort", "lastModified")
	values.Set("direction", "-1")
	values.Set("limit", "100")
	values.Set("full", "true")
	if maxParams := effectiveRadarMaxParams(criteria, machine); maxParams > 0 {
		values.Set("num_parameters", fmt.Sprintf("max:%.1fB", maxParams))
	}
	return "https://huggingface.co/api/models?" + values.Encode()
}

func fetchHFRadarResults(client *http.Client) ([]hfModelResult, error) {
	criteria, err := loadRadarCriteria()
	if err != nil {
		return nil, err
	}
	endpoint := radarQueryURL(criteria, machineFingerprint())
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "localctl-model-radar/3")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Hugging Face returned HTTP %s", resp.Status)
	}
	var results []hfModelResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}
	return results, nil
}

func runRadar(args []string, stdout, stderr io.Writer) int {
	if len(args) == 2 {
		return runRadarScan(false, stdout, stderr)
	}
	switch args[2] {
	case "scan", "refresh":
		jsonOutput := false
		for _, arg := range args[3:] {
			if arg == "--json" {
				jsonOutput = true
			} else {
				fmt.Fprintf(stderr, "unexpected radar scan argument: %s\n", arg)
				return 1
			}
		}
		return runRadarScan(jsonOutput, stdout, stderr)
	case "criteria":
		return runRadarCriteria(args, stdout, stderr)
	case "help", "--help", "-h":
		printRadarHelp(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown radar command: %s\n", args[2])
		return 1
	}
}

func printRadarHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "LocalCTL Model Radar")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "  localctl radar                         scan Hugging Face using your saved criteria")
	fmt.Fprintln(stdout, "  localctl radar scan [--json]           same scan, explicit form")
	fmt.Fprintln(stdout, "  localctl radar criteria                show criteria and machine-derived fit screen")
	fmt.Fprintln(stdout, "  localctl radar criteria set ...        change criteria")
	fmt.Fprintln(stdout, "  localctl radar criteria reset          restore conservative defaults")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Example:")
	fmt.Fprintln(stdout, "  localctl radar criteria set --fit=comfortable --use=coding,debugging,review --release-days=30")
}

func runRadarScan(jsonOutput bool, stdout, stderr io.Writer) int {
	criteria, err := loadRadarCriteria()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	models, err := discoverModels()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	results, err := fetchHFRadarResults(nil)
	if err != nil {
		fmt.Fprintf(stderr, "radar scan failed: %v\n", err)
		return 1
	}
	machine := machineFingerprint()
	candidates := rankRadarResults(results, criteria, machine, models, time.Now())
	limit := 8
	if len(candidates) < limit {
		limit = len(candidates)
	}
	candidates = candidates[:limit]
	if jsonOutput {
		payload := struct {
			Machine    machineFingerprintRecord `json:"machine"`
			Criteria   radarCriteria            `json:"criteria"`
			MaxParamsB float64                  `json:"effective_max_params_b,omitempty"`
			Candidates []radarCandidate         `json:"candidates"`
		}{machine, criteria, effectiveRadarMaxParams(criteria, machine), candidates}
		data, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}
	printRadarCriteriaSummary(stdout, criteria, machine)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "MODEL RADAR — candidates worth local testing")
	fmt.Fprintln(stdout, "Hugging Face metadata creates questions, not capability claims.")
	fmt.Fprintln(stdout)
	if len(candidates) == 0 {
		fmt.Fprintln(stdout, "No current candidates matched your criteria.")
		fmt.Fprintln(stdout, "Try: localctl radar criteria")
		return 0
	}
	fmt.Fprintln(stdout, "REPOSITORY                                      PARAM HINT  FIT SCREEN          MATCHES                 STATUS")
	for _, candidate := range candidates {
		params := "?"
		if candidate.ParametersB > 0 {
			params = fmt.Sprintf("~%.1fB", candidate.ParametersB)
		}
		matches := "—"
		if len(candidate.UseMatches) > 0 {
			matches = strings.Join(candidate.UseMatches, ",")
		}
		fmt.Fprintf(stdout, "%-47s %-11s %-19s %-23s %s\n", shorten(candidate.Repository, 47), params, candidate.Fit, shorten(matches, 23), candidate.LocalStatus)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "PARAM HINT comes from repository naming only. Hugging Face parameter metadata enforces the live fit screen.")
	fmt.Fprintln(stdout, "Pick a candidate for the existing audit queue:")
	fmt.Fprintln(stdout, "  localctl explore select <owner/model-GGUF>")
	fmt.Fprintln(stdout, "Then install it with your preferred model manager and let LocalCTL test it locally.")
	return 0
}

func runRadarCriteria(args []string, stdout, stderr io.Writer) int {
	if len(args) == 3 || (len(args) == 4 && args[3] == "show") {
		criteria, err := loadRadarCriteria()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		printRadarCriteriaSummary(stdout, criteria, machineFingerprint())
		return 0
	}
	if args[3] == "reset" {
		if err := resetRadarCriteria(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "Model Radar criteria reset to conservative machine-aware defaults.")
		return 0
	}
	if args[3] != "set" {
		fmt.Fprintln(stderr, "usage: localctl radar criteria [show|reset|set --fit=... --use=... ...]")
		return 1
	}
	criteria, err := loadRadarCriteria()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(args) == 4 {
		fmt.Fprintln(stderr, "criteria set requires at least one --option")
		return 1
	}
	for _, arg := range args[4:] {
		switch {
		case strings.HasPrefix(arg, "--fit="):
			criteria.Fit = strings.ToLower(strings.TrimPrefix(arg, "--fit="))
		case strings.HasPrefix(arg, "--use="):
			criteria.Uses = normalizeRadarUses([]string{strings.TrimPrefix(arg, "--use=")})
		case strings.HasPrefix(arg, "--max-params="):
			value := strings.TrimPrefix(arg, "--max-params=")
			if value == "auto" {
				criteria.MaxParamsB = 0
				continue
			}
			criteria.MaxParamsB, err = strconv.ParseFloat(value, 64)
		case strings.HasPrefix(arg, "--release-days="):
			criteria.ReleaseDays, err = strconv.Atoi(strings.TrimPrefix(arg, "--release-days="))
		case strings.HasPrefix(arg, "--min-downloads="):
			criteria.MinDownloads, err = strconv.Atoi(strings.TrimPrefix(arg, "--min-downloads="))
		case strings.HasPrefix(arg, "--license="):
			criteria.License = strings.ToLower(strings.TrimPrefix(arg, "--license="))
		default:
			fmt.Fprintf(stderr, "unexpected radar criteria option: %s\n", arg)
			return 1
		}
		if err != nil {
			fmt.Fprintf(stderr, "invalid radar criteria option %s: %v\n", arg, err)
			return 1
		}
	}
	if err := saveRadarCriteria(criteria); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "Model Radar criteria saved.")
	printRadarCriteriaSummary(stdout, criteria, machineFingerprint())
	return 0
}

func printRadarCriteriaSummary(stdout io.Writer, criteria radarCriteria, machine machineFingerprintRecord) {
	fmt.Fprintln(stdout, "MODEL RADAR CRITERIA")
	fmt.Fprintf(stdout, "fit:            %s\n", criteria.Fit)
	fmt.Fprintf(stdout, "use signals:    %s (ranking, not proof)\n", strings.Join(normalizeRadarUses(criteria.Uses), ", "))
	if criteria.ReleaseDays == 0 {
		fmt.Fprintln(stdout, "release age:    any")
	} else {
		fmt.Fprintf(stdout, "release age:    last %d days\n", criteria.ReleaseDays)
	}
	fmt.Fprintf(stdout, "min downloads:  %d\n", criteria.MinDownloads)
	fmt.Fprintf(stdout, "license filter: %s\n", criteria.License)
	maxParams := effectiveRadarMaxParams(criteria, machine)
	if criteria.MaxParamsB > 0 {
		fmt.Fprintf(stdout, "max parameters: %.1fB (explicit; enforced by Hugging Face metadata)\n", criteria.MaxParamsB)
	} else if maxParams > 0 {
		fmt.Fprintf(stdout, "max parameters: ~%.1fB (machine-derived; enforced by Hugging Face metadata)\n", maxParams)
	} else {
		fmt.Fprintln(stdout, "max parameters: no automatic cap (machine memory unavailable or fit=any)")
	}
	if memory := machineMemoryGB(machine); memory > 0 {
		fmt.Fprintf(stdout, "machine memory: %.1f GB\n", memory)
	}
	fmt.Fprintln(stdout, "The parameter threshold is still only a discovery screen. Local loading and measurement prove runtime fit.")
	if criteria.License != "any" {
		fmt.Fprintln(stdout, "License filtering uses Hugging Face metadata only; verify the model license before relying on it.")
	}
}
