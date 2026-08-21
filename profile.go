package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const localctlVersion = "0.5.0"

type profileDefinition struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Context     int     `json:"context"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	BuiltIn     bool    `json:"built_in,omitempty"`
}

var builtInProfiles = []profileDefinition{
	{ID: "default", Description: "Balanced learner profile and compatibility baseline.", Context: 2048, Temperature: 0, MaxTokens: 512, BuiltIn: true},
	{ID: "fast", Description: "Shorter completions for latency-sensitive utility work.", Context: 2048, Temperature: 0, MaxTokens: 256, BuiltIn: true},
	{ID: "long-context", Description: "Larger context probe; model/runtime fit still has to be observed.", Context: 8192, Temperature: 0, MaxTokens: 1024, BuiltIn: true},
}

func defaultProfile() profileDefinition {
	return builtInProfiles[0]
}

func profileRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".localctl", "profiles"), nil
}

func listProfiles() ([]profileDefinition, error) {
	profiles := append([]profileDefinition(nil), builtInProfiles...)
	root, err := profileRoot()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return profiles, nil
		}
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(root, entry.Name()))
		if readErr != nil {
			return nil, readErr
		}
		var profile profileDefinition
		if decodeErr := json.Unmarshal(data, &profile); decodeErr != nil {
			return nil, fmt.Errorf("decode profile %s: %w", entry.Name(), decodeErr)
		}
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	return profiles, nil
}

func resolveProfile(reference string) (profileDefinition, error) {
	if strings.TrimSpace(reference) == "" {
		return defaultProfile(), nil
	}
	profiles, err := listProfiles()
	if err != nil {
		return profileDefinition{}, err
	}
	needle := strings.ToLower(reference)
	var matches []profileDefinition
	for _, profile := range profiles {
		if strings.EqualFold(profile.ID, reference) {
			return profile, nil
		}
		if strings.Contains(strings.ToLower(profile.ID), needle) {
			matches = append(matches, profile)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return profileDefinition{}, fmt.Errorf("profile reference %q is ambiguous", reference)
	}
	return profileDefinition{}, fmt.Errorf("profile %q not found", reference)
}

func saveProfile(profile profileDefinition) error {
	if profile.ID == "" || strings.ContainsAny(profile.ID, "/\\") {
		return fmt.Errorf("profile ID must be a simple non-empty name")
	}
	if profile.Context <= 0 || profile.MaxTokens <= 0 || profile.Temperature < 0 {
		return fmt.Errorf("context/max-tokens must be positive and temperature cannot be negative")
	}
	for _, builtIn := range builtInProfiles {
		if profile.ID == builtIn.ID {
			return fmt.Errorf("cannot overwrite built-in profile %q", profile.ID)
		}
	}
	root, err := profileRoot()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, profile.ID+".json"), data, 0600)
}

func runProfiles(args []string, stdout, stderr io.Writer) int {
	profiles, err := listProfiles()
	if err != nil {
		fmt.Fprintf(stderr, "could not list profiles: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "PROFILE        CONTEXT   TEMP   MAX TOKENS   DESCRIPTION")
	for _, profile := range profiles {
		marker := ""
		if profile.BuiltIn {
			marker = " [built-in]"
		}
		fmt.Fprintf(stdout, "%-14s %-9d %-6g %-12d %s%s\n", profile.ID, profile.Context, profile.Temperature, profile.MaxTokens, profile.Description, marker)
	}
	return 0
}

func runProfile(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl profile <show|create> ...")
		return 1
	}
	switch args[2] {
	case "show":
		if len(args) < 4 {
			fmt.Fprintln(stderr, "usage: localctl profile show <profile>")
			return 1
		}
		profile, err := resolveProfile(args[3])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		data, _ := json.MarshalIndent(profile, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	case "create":
		if len(args) < 4 {
			fmt.Fprintln(stderr, "usage: localctl profile create <name> [--context=N] [--max-tokens=N] [--temperature=N]")
			return 1
		}
		profile := defaultProfile()
		profile.ID = args[3]
		profile.BuiltIn = false
		profile.Description = "User-defined LocalCTL runtime profile."
		for _, arg := range args[4:] {
			var parseErr error
			switch {
			case strings.HasPrefix(arg, "--context="):
				profile.Context, parseErr = strconv.Atoi(strings.TrimPrefix(arg, "--context="))
			case strings.HasPrefix(arg, "--max-tokens="):
				profile.MaxTokens, parseErr = strconv.Atoi(strings.TrimPrefix(arg, "--max-tokens="))
			case strings.HasPrefix(arg, "--temperature="):
				profile.Temperature, parseErr = strconv.ParseFloat(strings.TrimPrefix(arg, "--temperature="), 64)
			default:
				fmt.Fprintf(stderr, "unexpected profile argument: %s\n", arg)
				return 1
			}
			if parseErr != nil {
				fmt.Fprintf(stderr, "invalid profile argument %s: %v\n", arg, parseErr)
				return 1
			}
		}
		if err := saveProfile(profile); err != nil {
			fmt.Fprintf(stderr, "could not save profile: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "profile saved: %s\n", profile.ID)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown profile command: %s\n", args[2])
		return 1
	}
}
