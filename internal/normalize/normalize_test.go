package normalize

import (
	"testing"
)

func baseDoc() map[string]any {
	return map[string]any{
		"name": "app",
		"services": map[string]any{
			"built": map[string]any{
				"source": map[string]any{
					"repository": "https://github.com/example/app.git",
				},
			},
			"pulled": map[string]any{
				"source": map[string]any{
					"image": "example/pulled:1",
				},
			},
		},
		"stages": map[string]any{
			"prod": map[string]any{
				"target": map[string]any{"host": "edge-1"},
				"services": map[string]any{
					"built":  map[string]any{"revision": "abc", "exposure": "internal"},
					"pulled": map[string]any{"exposure": "internal"},
				},
			},
		},
	}
}

func TestCanonicalAppliesDefaults(t *testing.T) {
	canonical, err := Canonical(baseDoc())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	built := canonical["services"].(map[string]any)["built"].(map[string]any)
	source := built["source"].(map[string]any)
	build, ok := source["build"].(map[string]any)
	if !ok {
		t.Fatal("build defaults not created for repository service")
	}
	if build["context"] != "." || build["dockerfile"] != "Dockerfile" {
		t.Fatalf("build defaults wrong: %v", build)
	}
	if _, hasVolumes := built["volumes"]; !hasVolumes {
		t.Fatal("empty volumes list not materialized")
	}

	// Image services get no build block.
	pulled := canonical["services"].(map[string]any)["pulled"].(map[string]any)
	if _, hasBuild := pulled["source"].(map[string]any)["build"]; hasBuild {
		t.Fatal("image service must not receive build defaults")
	}

	stage := canonical["stages"].(map[string]any)["prod"].(map[string]any)
	if _, ok := stage["secrets"].(map[string]any); !ok {
		t.Fatal("empty stage secrets not materialized")
	}
	ss := stage["services"].(map[string]any)["built"].(map[string]any)
	if _, ok := ss["environment"].(map[string]any); !ok {
		t.Fatal("empty environment not materialized")
	}
	if _, ok := ss["mounts"].([]any); !ok {
		t.Fatal("empty mounts not materialized")
	}
}

func TestCanonicalDoesNotMutateInput(t *testing.T) {
	doc := baseDoc()
	if _, err := Canonical(doc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	built := doc["services"].(map[string]any)["built"].(map[string]any)
	if _, ok := built["source"].(map[string]any)["build"]; ok {
		t.Fatal("input was mutated")
	}
}

func TestCanonicalPreservesExplicitValues(t *testing.T) {
	doc := baseDoc()
	built := doc["services"].(map[string]any)["built"].(map[string]any)
	built["source"].(map[string]any)["build"] = map[string]any{
		"context":    "frontend",
		"dockerfile": "Dockerfile.dev",
	}
	canonical, err := Canonical(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	build := canonical["services"].(map[string]any)["built"].(map[string]any)["source"].(map[string]any)["build"].(map[string]any)
	if build["context"] != "frontend" || build["dockerfile"] != "Dockerfile.dev" {
		t.Fatalf("explicit values lost: %v", build)
	}
}

func TestCanonicalV1alpha2DefaultsPlacement(t *testing.T) {
	doc := baseDoc()
	doc["apiVersion"] = "appmanifest.mgconsulting.io/v1alpha2"
	delete(doc["stages"].(map[string]any)["prod"].(map[string]any), "target")
	canonical, err := Canonical(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	placement := canonical["stages"].(map[string]any)["prod"].(map[string]any)["placement"].(map[string]any)
	stage := canonical["stages"].(map[string]any)["prod"].(map[string]any)
	if stage["lifecycle"] != "active" {
		t.Fatalf("lifecycle default wrong: %v", stage["lifecycle"])
	}
	components := placement["requireComponents"].([]any)
	if len(components) != 1 || components[0] != "workload" {
		t.Fatalf("placement defaults wrong: %v", placement)
	}
}

func TestCanonicalV1alpha1DoesNotAddPlacement(t *testing.T) {
	canonical, err := Canonical(baseDoc())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stage := canonical["stages"].(map[string]any)["prod"].(map[string]any)
	if _, ok := stage["placement"]; ok {
		t.Fatal("v1alpha1 must not receive v1alpha2 placement defaults")
	}
}
