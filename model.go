package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type modelArtifact struct {
	ID   string
	Name string
	Path string
	Size int64
}

func discoverModels() ([]modelArtifact, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	return discoverModelsIn(filepath.Join(home, ".lmstudio", "models"))
}

func discoverModelsIn(root string) ([]modelArtifact, error) {
	var models []modelArtifact

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		if entry.IsDir() {
			return nil
		}

		if !strings.EqualFold(filepath.Ext(entry.Name()), ".gguf") {
			return nil
		}

		if strings.Contains(strings.ToLower(entry.Name()), "mmproj") {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		models = append(models, modelArtifact{
			ID:   modelSlug(name),
			Name: entry.Name(),
			Path: path,
			Size: info.Size(),
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})

	return models, nil
}

func modelSlug(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	lastDash := false

	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}

		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}

	return strings.Trim(b.String(), "-")
}

func resolveModel(reference string) (modelArtifact, error) {
	models, err := discoverModels()
	if err != nil {
		return modelArtifact{}, err
	}

	if len(models) == 0 {
		return modelArtifact{}, fmt.Errorf("no GGUF models found under ~/.lmstudio/models")
	}

	if reference == "" {
		for _, model := range models {
			if model.Name == modelID {
				return model, nil
			}
		}
		return models[0], nil
	}

	needle := strings.ToLower(reference)
	var matches []modelArtifact

	for _, model := range models {
		if strings.EqualFold(model.ID, reference) || strings.EqualFold(model.Name, reference) {
			return model, nil
		}

		haystack := strings.ToLower(model.ID + " " + model.Name + " " + model.Path)
		if strings.Contains(haystack, needle) {
			matches = append(matches, model)
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}

	if len(matches) == 0 {
		return modelArtifact{}, fmt.Errorf("no model matched %q", reference)
	}

	var names []string
	for _, match := range matches {
		names = append(names, match.ID)
	}

	return modelArtifact{}, fmt.Errorf("model reference %q is ambiguous: %s", reference, strings.Join(names, ", "))
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	div := int64(unit)
	exp := 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}
