package corpus

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

const validDoc = `
apiVersion: deployment.mgconsulting.io/v1alpha1
name: app
source:
  repository: https://github.com/example/app.git
container:
  httpPort: 80
stages:
  prod:
    revision: abc1234
    target: {host: edge-1}
    route: {hostname: app.example.com, exposure: public}
`

func TestDiscoverAndValidate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "one.yaml", validDoc)
	writeFile(t, dir, "two.yaml", validDoc)

	files, err := Discover(dir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %d", len(files))
	}
	docs, diags, err := ValidateAll(files)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("docs = %d", len(docs))
	}
	// Both files claim hostname app.example.com -> cross-document error.
	if !diags.HasErrors() {
		t.Fatal("expected duplicate hostname error")
	}
	found := false
	for _, d := range diags {
		if d.Code == "DUPLICATE_HOSTNAME" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing DUPLICATE_HOSTNAME, got: %s", diags.Human())
	}
}

func TestDiscoverRejectsYml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "one.yml", validDoc)
	if _, err := Discover(dir); err == nil {
		t.Fatal("expected error for .yml file")
	}
}

func TestDiscoverEmptyCorpusIsOperationalError(t *testing.T) {
	dir := t.TempDir()
	if _, err := Discover(dir); err == nil {
		t.Fatal("expected error for empty corpus")
	}
}

func TestDuplicateProjectNameDetected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", validDoc)
	writeFile(t, dir, "b.yaml", replaceIn(validDoc, "app.example.com", "other.example.com"))

	files, err := Discover(dir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	_, diags, err := ValidateAll(files)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == "DUPLICATE_PROJECT_NAME" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing DUPLICATE_PROJECT_NAME, got: %s", diags.Human())
	}
}

func replaceIn(s, old, new string) string {
	out := make([]byte, 0, len(s)+16)
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			out = append(out, new...)
			i += len(old)
		} else {
			out = append(out, s[i])
			i++
		}
	}
	return string(out)
}
