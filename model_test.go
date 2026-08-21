package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverModelsIn(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "vendor", "granite-4.1-8b-Q4_K_S.gguf"),
		filepath.Join(root, "vendor", "ministral-8b-Q4_K_M.gguf"),
		filepath.Join(root, "vendor", "mmproj-model-f16.gguf"),
		filepath.Join(root, "vendor", "notes.txt"),
	}

	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	models, err := discoverModelsIn(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 model artifacts, got %d: %#v", len(models), models)
	}
	if models[0].ID == "" || models[1].ID == "" {
		t.Fatalf("expected friendly model IDs")
	}
}

func TestModelSlug(t *testing.T) {
	got := modelSlug("Granite-4.1-8b-Q4_K_S")
	want := "granite-4-1-8b-q4-k-s"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
