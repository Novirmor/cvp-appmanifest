package validation

import (
	"testing"

	"github.com/MGconsulting/appmanifest/internal/diagnostic"
	"github.com/MGconsulting/appmanifest/internal/document"
)

type diagnosticList = diagnostic.List

func load(t *testing.T, yaml string) map[string]any {
	t.Helper()
	doc, err := document.Decode([]byte(yaml))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return doc
}

const minimalApp = `
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: app
services:
  web:
    source:
      repository: https://github.com/example/app.git
    httpPort: 8080
stages:
  prod:
    target: {host: edge-1}
    services:
      web:
        revision: abc1234
        exposure: public
        hostname: app.example.com
`

func hasCode(diags diagnosticList, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestValidateSchemaValid(t *testing.T) {
	diags := ValidateSchema(load(t, minimalApp))
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %s", diags.Human())
	}
}

func TestValidateSchemaUnsupportedAPIVersion(t *testing.T) {
	doc := load(t, "apiVersion: appmanifest.mgconsulting.io/v9\nname: app\n")
	diags := ValidateSchema(doc)
	if !hasCode(diags, "UNSUPPORTED_API_VERSION") {
		t.Fatalf("missing UNSUPPORTED_API_VERSION: %s", diags.Human())
	}
}

func TestValidateSchemaUnknownFieldRejected(t *testing.T) {
	doc := load(t, minimalApp+"\n      extra: true\n")
	diags := ValidateSchema(doc)
	if !diags.HasErrors() {
		t.Fatal("expected error for unknown field")
	}
}

func TestValidateSchemaBadPort(t *testing.T) {
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: app
services:
  web:
    source: {image: "example/app:1"}
    httpPort: 99999
stages:
  prod:
    target: {host: edge-1}
    services:
      web:
        exposure: internal
`)
	diags := ValidateSchema(doc)
	if !diags.HasErrors() {
		t.Fatal("expected error for invalid port")
	}
}

func TestValidateSchemaImageWithDigest(t *testing.T) {
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: app
services:
  web:
    source:
      image: ghcr.io/example/app@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
stages:
  prod:
    target: {host: edge-1}
    services:
      web:
        exposure: internal
`)
	diags := ValidateSchema(doc)
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %s", diags.Human())
	}
}

func TestValidateSchemaSecretRefIsOpaque(t *testing.T) {
	// The reference is store-agnostic: bare ids, URIs, and names are all
	// valid shapes for the shared schema; the controller owns the syntax.
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: app
services:
  db:
    source: {image: "postgres:17-alpine"}
stages:
  prod:
    target: {host: edge-1}
    secrets:
      password:
        secretRef: 01234567-89ab-cdef-0123-456789abcdef
    services:
      db:
        exposure: internal
        mounts:
          - secret: password
`)
	diags := ValidateSchema(doc)
	if diags.HasErrors() {
		t.Fatalf("unexpected errors for bare-id reference: %s", diags.Human())
	}
	if d := ValidateSemantic(doc); d.HasErrors() {
		t.Fatalf("unexpected semantic errors: %s", d.Human())
	}
}

func TestValidateSchemaUnsafeRevisionRejected(t *testing.T) {
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: app
services:
  web:
    source: {repository: "https://github.com/example/app.git"}
    httpPort: 80
stages:
  prod:
    target: {host: edge-1}
    services:
      web:
        revision: --option
        exposure: public
        hostname: app.example.com
`)
	diags := ValidateSchema(doc)
	if !diags.HasErrors() {
		t.Fatal("expected error for unsafe revision")
	}
}

func TestSemanticMissingStageService(t *testing.T) {
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: app
services:
  web:
    source: {image: "example/web:1"}
  worker:
    source: {image: "example/worker:1"}
stages:
  prod:
    target: {host: edge-1}
    services:
      web:
        exposure: internal
`)
	diags := ValidateSemantic(doc)
	if !hasCode(diags, "MISSING_STAGE_SERVICE") {
		t.Fatalf("missing MISSING_STAGE_SERVICE: %s", diags.Human())
	}
}

func TestSemanticUnknownStageService(t *testing.T) {
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: app
services:
  web:
    source: {image: "example/web:1"}
stages:
  prod:
    target: {host: edge-1}
    services:
      web:
        exposure: internal
      ghost:
        exposure: internal
`)
	diags := ValidateSemantic(doc)
	if !hasCode(diags, "UNKNOWN_STAGE_SERVICE") {
		t.Fatalf("missing UNKNOWN_STAGE_SERVICE: %s", diags.Human())
	}
}

func TestSemanticRevisionRules(t *testing.T) {
	diags := ValidateSemantic(load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: app
services:
  built:
    source: {repository: "https://github.com/example/app.git"}
    httpPort: 80
  pulled:
    source: {image: "example/pulled:1"}
stages:
  prod:
    target: {host: edge-1}
    services:
      built:
        exposure: internal
      pulled:
        revision: v1.2.3
        exposure: internal
`))
	foundMissing := false
	foundForbidden := false
	for _, d := range diags {
		if d.Code == "MISSING_REVISION" {
			foundMissing = true
		}
		if d.Code == "FORBIDDEN_REVISION" {
			foundForbidden = true
		}
	}
	if !foundMissing || !foundForbidden {
		t.Fatalf("expected MISSING_REVISION and FORBIDDEN_REVISION: %s", diags.Human())
	}
}

func TestSemanticRoutedServiceNeedsHostnameAndPort(t *testing.T) {
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: app
services:
  web:
    source: {image: "example/web:1"}
stages:
  prod:
    target: {host: edge-1}
    services:
      web:
        exposure: tailnet
`)
	diags := ValidateSemantic(doc)
	if !hasCode(diags, "MISSING_HOSTNAME") {
		t.Fatalf("missing MISSING_HOSTNAME: %s", diags.Human())
	}
	if !hasCode(diags, "MISSING_HTTP_PORT") {
		t.Fatalf("missing MISSING_HTTP_PORT: %s", diags.Human())
	}
}

func TestSemanticInternalServiceForbidsHostname(t *testing.T) {
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: app
services:
  db:
    source: {image: "postgres:17-alpine"}
stages:
  prod:
    target: {host: edge-1}
    services:
      db:
        exposure: internal
        hostname: db.example.com
`)
	diags := ValidateSemantic(doc)
	if !hasCode(diags, "FORBIDDEN_HOSTNAME") {
		t.Fatalf("missing FORBIDDEN_HOSTNAME: %s", diags.Human())
	}
}

func TestSemanticUnknownSecretReferences(t *testing.T) {
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: app
services:
  db:
    source: {image: "postgres:17-alpine"}
stages:
  prod:
    target: {host: edge-1}
    services:
      db:
        exposure: internal
        environment:
          POSTGRES_PASSWORD:
            secret: missing_password
        mounts:
          - secret: also_missing
`)
	diags := ValidateSemantic(doc)
	count := 0
	for _, d := range diags {
		if d.Code == "UNKNOWN_SECRET" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected two UNKNOWN_SECRET findings: %s", diags.Human())
	}
}

func TestSemanticUnknownVolumeReference(t *testing.T) {
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: app
services:
  db:
    source: {image: "postgres:17-alpine"}
    volumes:
      - name: undeclared
        path: /var/lib/postgresql/data
stages:
  prod:
    target: {host: edge-1}
    services:
      db:
        exposure: internal
`)
	diags := ValidateSemantic(doc)
	if !hasCode(diags, "UNKNOWN_VOLUME") {
		t.Fatalf("missing UNKNOWN_VOLUME: %s", diags.Human())
	}
}

func TestSemanticDuplicateHostnameAcrossStages(t *testing.T) {
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: app
services:
  web:
    source: {image: "example/web:1"}
    httpPort: 80
stages:
  prod:
    target: {host: edge-1}
    services:
      web:
        exposure: public
        hostname: app.example.com
  staging:
    target: {host: edge-2}
    services:
      web:
        exposure: tailnet
        hostname: app.example.com
`)
	diags := ValidateSemantic(doc)
	if !hasCode(diags, "DUPLICATE_HOSTNAME") {
		t.Fatalf("missing DUPLICATE_HOSTNAME: %s", diags.Human())
	}
}
