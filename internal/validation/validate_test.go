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

const minimalAppV2 = `
apiVersion: appmanifest.mgconsulting.io/v1alpha2
name: app
services:
  web:
    source:
      image: example/web:1
stages:
  prod:
    services:
      web:
        exposure: internal
        runtime: {relaxed: true}
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

func TestValidateSchemaV1alpha2PortableStage(t *testing.T) {
	diags := ValidateSchema(load(t, minimalAppV2))
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %s", diags.Human())
	}
}

func TestValidateSchemaV1alpha1DoesNotAcceptV1alpha2Fields(t *testing.T) {
	doc := load(t, minimalApp+"\n    placement: {}\n")
	if diags := ValidateSchema(doc); !diags.HasErrors() {
		t.Fatal("expected v1alpha1 to reject placement")
	}
}

func TestValidateSchemaV1alpha2DoesNotAcceptTarget(t *testing.T) {
	doc := load(t, minimalAppV2+"\n    target: {host: edge-1}\n")
	if diags := ValidateSchema(doc); !diags.HasErrors() {
		t.Fatal("expected v1alpha2 to reject target")
	}
}

func TestValidateSchemaV1alpha2PlacementRequiresWorkload(t *testing.T) {
	doc := load(t, minimalAppV2+"\n    placement:\n      requireComponents: [postgres]\n")
	if diags := ValidateSchema(doc); !diags.HasErrors() {
		t.Fatal("expected placement without workload to be rejected")
	}
}

func TestValidateSchemaV1alpha2PlatformRequestsAreEmptyObjects(t *testing.T) {
	doc := load(t, minimalAppV2+"\n    platform:\n      postgres:\n        database: true\n")
	if diags := ValidateSchema(doc); !diags.HasErrors() {
		t.Fatal("expected non-empty platform request to be rejected")
	}
}

func TestValidateSchemaV1alpha2RedisRequiresPasswordSecret(t *testing.T) {
	doc := load(t, minimalAppV2+"\n    platform:\n      redis: {}\n")
	if diags := ValidateSchema(doc); !diags.HasErrors() {
		t.Fatal("expected Redis request without passwordSecret to be rejected")
	}
}

func TestValidateSchemaV1alpha2PlatformRequiresAnEngine(t *testing.T) {
	doc := load(t, minimalAppV2+"\n    platform: {}\n")
	if diags := ValidateSchema(doc); !diags.HasErrors() {
		t.Fatal("expected empty platform request to be rejected")
	}
}

func TestValidateSchemaV1alpha2Lifecycle(t *testing.T) {
	doc := load(t, minimalAppV2+"\n    lifecycle: retiring\n")
	if diags := ValidateSchema(doc); diags.HasErrors() {
		t.Fatalf("expected lifecycle to be accepted: %s", diags.Human())
	}
	doc = load(t, minimalAppV2+"\n    lifecycle: deleted\n")
	if diags := ValidateSchema(doc); !diags.HasErrors() {
		t.Fatal("expected unknown lifecycle to be rejected")
	}
}

func TestSemanticV1alpha2RedisPasswordSecretMustExist(t *testing.T) {
	doc := load(t, minimalAppV2+"\n    platform:\n      redis: {passwordSecret: redis_password}\n")
	if diags := ValidateSemantic(doc); !hasCode(diags, "MISSING_REDIS_PASSWORD_SECRET") {
		t.Fatalf("expected MISSING_REDIS_PASSWORD_SECRET: %s", diags.Human())
	}
}

func TestValidateSchemaV1alpha2DataServicesAreConstrained(t *testing.T) {
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha2
name: app
services:
  web:
    source: {image: example/web:1}
    dataServices: [openbao]
stages:
  prod:
    services:
      web: {exposure: internal}
`)
	if diags := ValidateSchema(doc); !diags.HasErrors() {
		t.Fatal("expected unknown data service to be rejected")
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
        secretRef: example-secret-ref-1
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

func TestSemanticV1alpha2DataServiceDoesNotRequireTenancy(t *testing.T) {
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha2
name: app
services:
  web:
    source: {image: example/web:1}
    dataServices: [postgres, redis]
stages:
  prod:
    services:
      web: {exposure: internal, runtime: {relaxed: true}}
`)
	if diags := ValidateSemantic(doc); diags.HasErrors() {
		t.Fatalf("data network access must not create a tenancy requirement: %s", diags.Human())
	}
}

func TestSemanticV1alpha2DataServicesMayUseRequestedTenancy(t *testing.T) {
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha2
name: app
services:
  web:
    source: {image: example/web:1}
    dataServices: [postgres, rabbitmq, redis]
stages:
  prod:
    platform:
      postgres: {}
      rabbitmq: {}
      redis: {passwordSecret: redis_password}
    secrets:
      redis_password: {secretRef: opaque-reference}
    services:
      web: {exposure: internal, runtime: {relaxed: true}}
`)
	if diags := ValidateSemantic(doc); diags.HasErrors() {
		t.Fatalf("unexpected semantic errors: %s", diags.Human())
	}
}

func TestSemanticV1alpha2RuntimeAndRouteGuards(t *testing.T) {
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha2
name: app
services:
  web:
    source: {image: example/web:1}
stages:
  prod:
    services:
      web:
        exposure: internal
        forwardAuth: true
        smokePath: /
`)
	diags := ValidateSemantic(doc)
	for _, code := range []string{"MISSING_NONROOT_RUNTIME", "FORWARDAUTH_REQUIRES_TAILNET_ROUTE", "SMOKE_REQUIRES_ROUTE"} {
		if !hasCode(diags, code) {
			t.Fatalf("missing %s: %s", code, diags.Human())
		}
	}
}

func TestSemanticV1alpha2RejectsUnusedPlatformRequest(t *testing.T) {
	doc := load(t, `
apiVersion: appmanifest.mgconsulting.io/v1alpha2
name: app
services:
  web:
    source: {image: example/web:1}
stages:
  prod:
    platform:
      postgres: {}
    services:
      web: {exposure: internal, runtime: {relaxed: true}}
`)
	if diags := ValidateSemantic(doc); !hasCode(diags, "UNUSED_PLATFORM_SERVICE") {
		t.Fatalf("missing UNUSED_PLATFORM_SERVICE: %s", diags.Human())
	}
}
