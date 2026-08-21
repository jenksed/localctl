package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestV05LabLeadsWithLearning(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runV05Lab(&stdout, &stderr); code != 0 {
		t.Fatalf("v0.5 lab failed: %d: %s", code, stderr.String())
	}
	for _, want := range []string{
		"Learn What Matters Next",
		"LEARN WHAT YOUR SYSTEM CAN DO",
		"WHAT SHOULD THE LAB LEARN NEXT?",
		"localctl radar",
		"External metadata only gives the lab new questions to test",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("v0.5 lab output missing %q:\n%s", want, stdout.String())
		}
	}
}
