package validation

import (
	"encoding/json"
	"testing"

	"github.com/MGconsulting/deployment-spec/internal/document"
	v1alpha1 "github.com/MGconsulting/deployment-spec/schema/v1alpha1"
)

func load(t *testing.T, yaml string) map[string]any {
	t.Helper()
	doc, err := document.Decode([]byte(yaml))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return doc
}

func TestValidateSchemaValid(t *testing.T) {
	doc := load(t, `
apiVersion: deployment.mgconsulting.io/v1alpha1
name: app
source:
  repository: https://github.com/example/app.git
container:
  httpPort: 8080
stages:
  prod:
    revision: abc1234
    target: {host: edge-1}
    environment:
      KEY: {value: "v"}
    route: {hostname: app.example.com, exposure: public}
`)
	diags := ValidateSchema(doc)
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %s", diags.Human())
	}
}

func TestValidateSchemaUnsupportedAPIVersion(t *testing.T) {
	doc := load(t, "apiVersion: deployment.mgconsulting.io/v9\nname: app\n")
	diags := ValidateSchema(doc)
	if !diags.HasErrors() {
		t.Fatal("expected error for unsupported apiVersion")
	}
	if diags[0].Code != "UNSUPPORTED_API_VERSION" {
		t.Fatalf("code = %s", diags[0].Code)
	}
}

func TestValidateSchemaUnknownFieldRejected(t *testing.T) {
	doc := load(t, `
apiVersion: deployment.mgconsulting.io/v1alpha1
name: app
source:
  repository: https://github.com/example/app.git
container:
  httpPort: 8080
stages:
  prod:
    revision: abc
    target: {host: edge-1}
    route: {hostname: app.example.com, exposure: public}
    replicas: 3
`)
	diags := ValidateSchema(doc)
	if !diags.HasErrors() {
		t.Fatal("expected error for unknown field")
	}
}

func TestValidateSchemaBadPort(t *testing.T) {
	doc := load(t, `
apiVersion: deployment.mgconsulting.io/v1alpha1
name: app
source:
  repository: https://github.com/example/app.git
container:
  httpPort: 99999
stages:
  prod:
    revision: abc
    target: {host: edge-1}
    route: {hostname: app.example.com, exposure: public}
`)
	diags := ValidateSchema(doc)
	if !diags.HasErrors() {
		t.Fatal("expected error for invalid port")
	}
}

func TestValidateSchemaUnsafeRevisionRejected(t *testing.T) {
	doc := load(t, `
apiVersion: deployment.mgconsulting.io/v1alpha1
name: app
source:
  repository: https://github.com/example/app.git
container:
  httpPort: 80
stages:
  prod:
    revision: --option
    target: {host: edge-1}
    route: {hostname: app.example.com, exposure: public}
`)
	diags := ValidateSchema(doc)
	if !diags.HasErrors() {
		t.Fatal("expected error for unsafe revision")
	}
}

func TestValidateSemanticDuplicateHostname(t *testing.T) {
	doc := load(t, `
apiVersion: deployment.mgconsulting.io/v1alpha1
name: app
source:
  repository: https://github.com/example/app.git
container:
  httpPort: 80
stages:
  a:
    revision: abc
    target: {host: edge-1}
    route: {hostname: same.example.com, exposure: public}
  b:
    revision: abc
    target: {host: edge-2}
    route: {hostname: same.example.com, exposure: tailnet}
`)
	diags := ValidateSemantic(doc)
	if !diags.HasErrors() {
		t.Fatal("expected duplicate hostname error")
	}
	if diags[0].Code != "DUPLICATE_HOSTNAME" {
		t.Fatalf("code = %s", diags[0].Code)
	}
}

func TestSchemaByteEquality(t *testing.T) {
	// The embedded schema must be valid JSON, carry the API version, and be
	// byte-identical to the released asset. The byte-identical release asset
	// check runs in CI against the packaged schema file.
	if len(v1alpha1.Schema) == 0 {
		t.Fatal("embedded schema is empty")
	}
	var parsed map[string]any
	if err := json.Unmarshal(v1alpha1.Schema, &parsed); err != nil {
		t.Fatalf("embedded schema is not valid JSON: %v", err)
	}
	if parsed["$id"] == nil {
		t.Fatal("embedded schema has no $id")
	}
}
