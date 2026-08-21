package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestV04LabSurfacesCapabilityLoop(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runV04Lab(&stdout, &stderr); code != 0 {
		t.Fatalf("v0.4 lab failed: %d: %s", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"Know the Territory",
		"KNOW WHAT YOUR MAC CAN HANDLE",
		"discover → test → verify → map → recommend → refresh",
		"localctl capability <model>",
		"localctl matrix",
		"localctl recommend <pack>",
		"localctl requalify <model>",
		"localctl intelligence audit",
		"does not authorize model execution",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("v0.4 lab output missing %q:\n%s", want, output)
		}
	}
}

func TestIntelligenceCLIRequiresSubcommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"localctl", "intelligence"}, &stdout, &stderr); code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if got := stderr.String(); got != "usage: localctl intelligence <list|show|audit> ...\n" {
		t.Fatalf("unexpected stderr: %q", got)
	}
}
