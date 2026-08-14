package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunVersion(t *testing.T) {
	if code := run([]string{"version"}); code != exitOK {
		t.Fatalf("version exit = %d", code)
	}
}

func TestRunUsage(t *testing.T) {
	if code := run(nil); code != exitOper {
		t.Fatalf("usage exit = %d", code)
	}
	if code := run([]string{"bogus"}); code != exitOper {
		t.Fatalf("unknown command exit = %d", code)
	}
}

func TestRunValidateValidFile(t *testing.T) {
	path := writeTempYAML(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha1
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
`)
	if code := run([]string{"validate", "--file", path}); code != exitOK {
		t.Fatalf("validate exit = %d", code)
	}
}

func TestRunValidateInvalidFile(t *testing.T) {
	path := writeTempYAML(t, "apiVersion: appmanifest.mgconsulting.io/v9\nname: app\n")
	if code := run([]string{"validate", "--file", path}); code != exitInvalid {
		t.Fatalf("validate exit = %d", code)
	}
}

func TestRunValidateMissingArg(t *testing.T) {
	if code := run([]string{"validate"}); code != exitOper {
		t.Fatalf("validate missing-arg exit = %d", code)
	}
}

func TestRunNormalizeAppliesDefaults(t *testing.T) {
	path := writeTempYAML(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha1
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
`)
	if code := run([]string{"normalize", "--file", path}); code != exitOK {
		t.Fatalf("normalize exit = %d", code)
	}
}

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "doc.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}
