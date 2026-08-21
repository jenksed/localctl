package main

import "testing"

func TestInfrastructureCatalogAvailableOnBuildTarget(t *testing.T) {
	if got := len(linuxInvestigationExerciseCatalog()); got != 34 {
		t.Fatalf("expected 34 Linux investigation scenarios, got %d", got)
	}
	if got := len(dockerInvestigationExerciseCatalog()); got != 33 {
		t.Fatalf("expected 33 Docker investigation scenarios, got %d", got)
	}
	if got := len(kubernetesInvestigationExerciseCatalog()); got != 33 {
		t.Fatalf("expected 33 Kubernetes investigation scenarios, got %d", got)
	}
}
