package normalize

import (
	"testing"
)

func TestCanonicalAppliesDefaults(t *testing.T) {
	doc := map[string]any{
		"name": "app",
		"source": map[string]any{
			"repository": "https://github.com/example/app.git",
		},
		"container": map[string]any{"httpPort": int64(80)},
		"stages": map[string]any{
			"prod": map[string]any{
				"revision": "abc",
			},
		},
	}
	canonical, err := Canonical(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	build := canonical["source"].(map[string]any)["build"].(map[string]any)
	if build["context"] != "." || build["dockerfile"] != "Dockerfile" {
		t.Fatalf("defaults not applied: %v", build)
	}
	stage := canonical["stages"].(map[string]any)["prod"].(map[string]any)
	if _, ok := stage["environment"].(map[string]any); !ok {
		t.Fatal("environment default not applied")
	}
}

func TestCanonicalDoesNotMutateInput(t *testing.T) {
	doc := map[string]any{
		"name": "app",
		"source": map[string]any{
			"repository": "https://github.com/example/app.git",
		},
		"container": map[string]any{"httpPort": int64(80)},
		"stages":    map[string]any{},
	}
	if _, err := Canonical(doc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	source := doc["source"].(map[string]any)
	if _, ok := source["build"]; ok {
		t.Fatal("input was mutated")
	}
}

func TestCanonicalPreservesExplicitValues(t *testing.T) {
	doc := map[string]any{
		"name": "app",
		"source": map[string]any{
			"repository": "https://github.com/example/app.git",
			"build":      map[string]any{"context": "frontend", "dockerfile": "Dockerfile.dev"},
		},
		"container": map[string]any{"httpPort": int64(8080)},
		"stages":    map[string]any{},
	}
	canonical, err := Canonical(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	build := canonical["source"].(map[string]any)["build"].(map[string]any)
	if build["context"] != "frontend" || build["dockerfile"] != "Dockerfile.dev" {
		t.Fatalf("explicit values lost: %v", build)
	}
}
