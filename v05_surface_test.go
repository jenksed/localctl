package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestV05HelpSurfacesNextAndCriteriaAwareRadar(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"localctl", "help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help failed: %d: %s", code, stderr.String())
	}
	for _, want := range []string{
		"localctl next",
		"localctl radar",
		"localctl radar criteria",
		"localctl capability",
		"localctl intelligence audit",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRadarCriteriaCLIStoresSimpleOperatorChoices(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"localctl", "radar", "criteria", "set",
		"--fit=stretch",
		"--use=coding,review",
		"--release-days=30",
		"--license=permissive",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("criteria set failed: %d: %s", code, stderr.String())
	}
	criteria, err := loadRadarCriteria()
	if err != nil {
		t.Fatal(err)
	}
	if criteria.Fit != "stretch" || criteria.ReleaseDays != 30 || criteria.License != "permissive" {
		t.Fatalf("unexpected saved criteria: %#v", criteria)
	}
	if got := strings.Join(criteria.Uses, ","); got != "coding,review" {
		t.Fatalf("unexpected uses: %s", got)
	}
	if machineFingerprint().MemoryBytes > 0 {
		output := stdout.String()
		if !strings.Contains(output, "machine-derived") || !strings.Contains(output, "discovery screen") {
			t.Fatalf("criteria output should expose the machine-derived discovery-screen boundary:\n%s", output)
		}
	}
}

func TestNextOfflineWorksWithoutHuggingFace(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"localctl", "next", "--offline"}, &stdout, &stderr); code != 0 {
		t.Fatalf("offline next failed: %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ecosystem radar: offline") {
		t.Fatalf("offline status not visible:\n%s", stdout.String())
	}
}
