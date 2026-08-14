// Package validation applies the immutable schema and per-document semantic
// checks to a decoded deployment document.
package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/MGconsulting/deployment-spec/internal/diagnostic"
	v1alpha1 "github.com/MGconsulting/deployment-spec/schema/v1alpha1"
)

// printer formats schema error kinds deterministically.
var printer = message.NewPrinter(language.English)

// SupportedAPIVersions lists the API versions this validator release accepts.
var SupportedAPIVersions = []string{v1alpha1.APIVersion}

var hostnamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// forbiddenLoader blocks every external $ref resolution; references must be
// local and pre-registered.
type forbiddenLoader struct{}

func (forbiddenLoader) Load(_ string) (any, error) {
	return nil, errors.New("network $ref resolution is forbidden")
}

func compileSchema() (*jsonschema.Schema, error) {
	var schemaDoc any
	if err := json.Unmarshal(v1alpha1.Schema, &schemaDoc); err != nil {
		return nil, fmt.Errorf("decode schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	c.UseLoader(forbiddenLoader{})
	if err := c.AddResource(v1alpha1.APIVersion, schemaDoc); err != nil {
		return nil, fmt.Errorf("register schema: %w", err)
	}
	return c.Compile(v1alpha1.APIVersion)
}

// ValidateSchema checks the decoded document against the v1alpha1 schema.
func ValidateSchema(doc map[string]any) diagnostic.List {
	var out diagnostic.List

	apiVersion, _ := doc["apiVersion"].(string)
	if apiVersion != v1alpha1.APIVersion {
		out.Add(diagnostic.Diagnostic{
			Code:     "UNSUPPORTED_API_VERSION",
			Severity: diagnostic.SeverityError,
			Path:     "$.apiVersion",
			Message:  fmt.Sprintf("unsupported apiVersion %q; supported: %v", apiVersion, SupportedAPIVersions),
		})
		return out
	}

	sch, err := compileSchema()
	if err != nil {
		out.Add(diagnostic.Diagnostic{
			Code:     "INTERNAL_SCHEMA_ERROR",
			Severity: diagnostic.SeverityError,
			Message:  err.Error(),
		})
		return out
	}

	if err := sch.Validate(doc); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			walkCauses(ve, &out)
		} else {
			out.Add(diagnostic.Diagnostic{
				Code:     "SCHEMA_VIOLATION",
				Severity: diagnostic.SeverityError,
				Message:  err.Error(),
			})
		}
	}
	return out
}

func walkCauses(ve *jsonschema.ValidationError, out *diagnostic.List) {
	if len(ve.Causes) == 0 {
		*out = append(*out, diagnostic.Diagnostic{
			Code:     "SCHEMA_VIOLATION",
			Severity: diagnostic.SeverityError,
			Path:     instancePath(ve.InstanceLocation),
			Message:  ve.ErrorKind.LocalizedString(printer),
		})
		return
	}
	for _, cause := range ve.Causes {
		walkCauses(cause, out)
	}
}

func instancePath(segments []string) string {
	if len(segments) == 0 {
		return "$"
	}
	var b strings.Builder
	b.WriteByte('$')
	for _, s := range segments {
		if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
			b.WriteString(s)
		} else {
			b.WriteByte('.')
			b.WriteString(s)
		}
	}
	return b.String()
}

// ValidateSemantic runs per-document checks that JSON Schema cannot express:
// no stage may claim a hostname another stage of the same document claims.
func ValidateSemantic(doc map[string]any) diagnostic.List {
	var out diagnostic.List
	stages, _ := doc["stages"].(map[string]any)
	claimed := make(map[string]string, len(stages))
	for stageName, raw := range stages {
		stage, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		route, ok := stage["route"].(map[string]any)
		if !ok {
			continue
		}
		hostname, ok := route["hostname"].(string)
		if !ok || !hostnamePattern.MatchString(hostname) {
			continue
		}
		if other, exists := claimed[hostname]; exists {
			out.Add(diagnostic.Diagnostic{
				Code:     "DUPLICATE_HOSTNAME",
				Severity: diagnostic.SeverityError,
				Path:     "$.stages." + stageName + ".route.hostname",
				Message:  fmt.Sprintf("hostname %q is already claimed by stage %q of this document", hostname, other),
			})
			continue
		}
		claimed[hostname] = stageName
	}
	return out
}
