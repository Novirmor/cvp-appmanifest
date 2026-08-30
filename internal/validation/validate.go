// Package validation applies the immutable schema and per-document semantic
// checks to a decoded application manifest.
package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/MGconsulting/appmanifest/internal/diagnostic"
	v1alpha1 "github.com/MGconsulting/appmanifest/schema/v1alpha1"
	v1alpha2 "github.com/MGconsulting/appmanifest/schema/v1alpha2"
)

// SupportedAPIVersions lists the API versions this validator release accepts.
var SupportedAPIVersions = []string{v1alpha1.APIVersion, v1alpha2.APIVersion}

var hostnamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// printer formats schema error kinds deterministically.
var printer = message.NewPrinter(language.English)

// forbiddenLoader blocks every external $ref resolution; references must be
// local and pre-registered.
type forbiddenLoader struct{}

func (forbiddenLoader) Load(_ string) (any, error) {
	return nil, errors.New("network $ref resolution is forbidden")
}

func compileSchema(apiVersion string, schema []byte) (*jsonschema.Schema, error) {
	var schemaDoc any
	if err := json.Unmarshal(schema, &schemaDoc); err != nil {
		return nil, fmt.Errorf("decode schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	c.UseLoader(forbiddenLoader{})
	if err := c.AddResource(apiVersion, schemaDoc); err != nil {
		return nil, fmt.Errorf("register schema: %w", err)
	}
	return c.Compile(apiVersion)
}

// ValidateSchema checks the decoded manifest against the schema matching its apiVersion.
func ValidateSchema(doc map[string]any) diagnostic.List {
	var out diagnostic.List

	apiVersion, _ := doc["apiVersion"].(string)
	var schema []byte
	switch apiVersion {
	case v1alpha1.APIVersion:
		schema = v1alpha1.Schema
	case v1alpha2.APIVersion:
		schema = v1alpha2.Schema
	default:
		out.Add(diagnostic.Diagnostic{
			Code:     "UNSUPPORTED_API_VERSION",
			Severity: diagnostic.SeverityError,
			Path:     "$.apiVersion",
			Message:  fmt.Sprintf("unsupported apiVersion %q; supported: %v", apiVersion, SupportedAPIVersions),
		})
		return out
	}

	sch, err := compileSchema(apiVersion, schema)
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

// ValidateSemantic runs per-document checks that JSON Schema cannot express.
func ValidateSemantic(doc map[string]any) diagnostic.List {
	apiVersion, _ := doc["apiVersion"].(string)
	out := validateCommon(doc)
	if apiVersion == v1alpha2.APIVersion {
		out = append(out, validateV1alpha2(doc)...)
	}
	return out
}

func validateCommon(doc map[string]any) diagnostic.List {
	var out diagnostic.List

	services, _ := doc["services"].(map[string]any)
	volumes, _ := doc["volumes"].(map[string]any)
	stages, _ := doc["stages"].(map[string]any)

	// Service volume mounts must reference declared top-level volumes.
	for svcName, raw := range services {
		svc, _ := raw.(map[string]any)
		mounts, _ := svc["volumes"].([]any)
		for i, m := range mounts {
			mount, _ := m.(map[string]any)
			name, _ := mount["name"].(string)
			if _, ok := volumes[name]; !ok {
				out.Add(diagnostic.Diagnostic{
					Code:     "UNKNOWN_VOLUME",
					Severity: diagnostic.SeverityError,
					Path:     fmt.Sprintf("$.services.%s.volumes[%d].name", svcName, i),
					Message:  fmt.Sprintf("volume %q is not declared under $.volumes", name),
				})
			}
		}
	}

	claimed := hostnameClaim{}
	for stageName, raw := range stages {
		stage, _ := raw.(map[string]any)
		stageServices, _ := stage["services"].(map[string]any)
		secrets, _ := stage["secrets"].(map[string]any)

		// Every application service must be placed in every stage, and a stage
		// may not place services the application does not declare.
		for svcName := range services {
			if _, ok := stageServices[svcName]; !ok {
				out.Add(diagnostic.Diagnostic{
					Code:     "MISSING_STAGE_SERVICE",
					Severity: diagnostic.SeverityError,
					Path:     fmt.Sprintf("$.stages.%s.services.%s", stageName, svcName),
					Message:  fmt.Sprintf("stage %q does not place service %q; the complete application is deployed per stage", stageName, svcName),
				})
			}
		}
		for svcName := range stageServices {
			if _, ok := services[svcName]; !ok {
				out.Add(diagnostic.Diagnostic{
					Code:     "UNKNOWN_STAGE_SERVICE",
					Severity: diagnostic.SeverityError,
					Path:     fmt.Sprintf("$.stages.%s.services.%s", stageName, svcName),
					Message:  fmt.Sprintf("stage %q places service %q which the application does not declare", stageName, svcName),
				})
			}
		}

		for svcName, rawSS := range stageServices {
			ss, _ := rawSS.(map[string]any)
			svc, _ := services[svcName].(map[string]any)
			source, _ := svc["source"].(map[string]any)
			_, gitSource := source["repository"]
			_, imageSource := source["image"]

			// Revision rules: git-built services need one per stage; image
			// services must not carry one.
			_, hasRevision := ss["revision"]
			if gitSource && !hasRevision {
				out.Add(diagnostic.Diagnostic{
					Code:     "MISSING_REVISION",
					Severity: diagnostic.SeverityError,
					Path:     fmt.Sprintf("$.stages.%s.services.%s.revision", stageName, svcName),
					Message:  fmt.Sprintf("service %q is built from a repository and requires a revision in stage %q", svcName, stageName),
				})
			}
			if imageSource && hasRevision {
				out.Add(diagnostic.Diagnostic{
					Code:     "FORBIDDEN_REVISION",
					Severity: diagnostic.SeverityError,
					Path:     fmt.Sprintf("$.stages.%s.services.%s.revision", stageName, svcName),
					Message:  fmt.Sprintf("service %q uses a prebuilt image and must not declare a revision", svcName),
				})
			}

			// Exposure and hostname conditionals.
			exposure, _ := ss["exposure"].(string)
			hostname, hasHostname := ss["hostname"].(string)
			if exposure == "public" || exposure == "tailnet" {
				if !hasHostname || !hostnamePattern.MatchString(hostname) {
					out.Add(diagnostic.Diagnostic{
						Code:     "MISSING_HOSTNAME",
						Severity: diagnostic.SeverityError,
						Path:     fmt.Sprintf("$.stages.%s.services.%s.hostname", stageName, svcName),
						Message:  fmt.Sprintf("service %q is exposed %q in stage %q and requires a hostname", svcName, exposure, stageName),
					})
				} else {
					claimed.add(hostname, fmt.Sprintf("service %q of stage %q", svcName, stageName), fmt.Sprintf("$.stages.%s.services.%s.hostname", stageName, svcName), &out)
				}
				if _, hasPort := svc["httpPort"]; !hasPort {
					out.Add(diagnostic.Diagnostic{
						Code:     "MISSING_HTTP_PORT",
						Severity: diagnostic.SeverityError,
						Path:     fmt.Sprintf("$.services.%s.httpPort", svcName),
						Message:  fmt.Sprintf("service %q is routed in stage %q and must declare services.%s.httpPort", svcName, stageName, svcName),
					})
				}
			}
			if exposure == "internal" && hasHostname {
				out.Add(diagnostic.Diagnostic{
					Code:     "FORBIDDEN_HOSTNAME",
					Severity: diagnostic.SeverityError,
					Path:     fmt.Sprintf("$.stages.%s.services.%s.hostname", stageName, svcName),
					Message:  fmt.Sprintf("service %q is internal in stage %q and must not declare a hostname", svcName, stageName),
				})
			}

			// Env entries referencing secrets and file mounts must resolve
			// against the stage's declared secrets.
			environment, _ := ss["environment"].(map[string]any)
			for envName, rawEntry := range environment {
				entry, _ := rawEntry.(map[string]any)
				secretName, ok := entry["secret"].(string)
				if !ok {
					continue
				}
				if _, ok := secrets[secretName]; !ok {
					out.Add(diagnostic.Diagnostic{
						Code:     "UNKNOWN_SECRET",
						Severity: diagnostic.SeverityError,
						Path:     fmt.Sprintf("$.stages.%s.services.%s.environment.%s.secret", stageName, svcName, envName),
						Message:  fmt.Sprintf("secret %q is not declared in stage %q", secretName, stageName),
					})
				}
			}
			mounts, _ := ss["mounts"].([]any)
			for i, m := range mounts {
				mount, _ := m.(map[string]any)
				secretName, _ := mount["secret"].(string)
				if _, ok := secrets[secretName]; !ok {
					out.Add(diagnostic.Diagnostic{
						Code:     "UNKNOWN_SECRET",
						Severity: diagnostic.SeverityError,
						Path:     fmt.Sprintf("$.stages.%s.services.%s.mounts[%d].secret", stageName, svcName, i),
						Message:  fmt.Sprintf("secret %q is not declared in stage %q", secretName, stageName),
					})
				}
			}
		}
	}
	return out
}

func validateV1alpha2(doc map[string]any) diagnostic.List {
	var out diagnostic.List
	services, _ := doc["services"].(map[string]any)
	stages, _ := doc["stages"].(map[string]any)
	for stageName, rawStage := range stages {
		stage, _ := rawStage.(map[string]any)
		platform, _ := stage["platform"].(map[string]any)
		secrets, _ := stage["secrets"].(map[string]any)
		if redis, ok := platform["redis"].(map[string]any); ok {
			passwordSecret, _ := redis["passwordSecret"].(string)
			if _, exists := secrets[passwordSecret]; !exists {
				out.Add(diagnostic.Diagnostic{
					Code:     "MISSING_REDIS_PASSWORD_SECRET",
					Severity: diagnostic.SeverityError,
					Path:     fmt.Sprintf("$.stages.%s.platform.redis.passwordSecret", stageName),
					Message:  fmt.Sprintf("Redis request in stage %q references undeclared stage secret %q", stageName, passwordSecret),
				})
			}
		}
		stageServices, _ := stage["services"].(map[string]any)
		lifecycle, _ := stage["lifecycle"].(string)
		for serviceName, rawStageService := range stageServices {
			stageService, _ := rawStageService.(map[string]any)
			runtime, _ := stageService["runtime"].(map[string]any)
			relaxed, _ := runtime["relaxed"].(bool)
			user, hasUser := runtime["user"].(string)
			if lifecycle != "retiring" && lifecycle != "purge" && !relaxed && (!hasUser || user == "") {
				out.Add(diagnostic.Diagnostic{
					Code:     "MISSING_NONROOT_RUNTIME",
					Severity: diagnostic.SeverityError,
					Path:     fmt.Sprintf("$.stages.%s.services.%s.runtime", stageName, serviceName),
					Message:  fmt.Sprintf("active v1alpha2 service %q in stage %q must declare runtime.user or runtime.relaxed: true", serviceName, stageName),
				})
			}
			if _, hasSmoke := stageService["smokePath"]; hasSmoke {
				exposure, _ := stageService["exposure"].(string)
				if exposure == "internal" {
					out.Add(diagnostic.Diagnostic{
						Code:     "SMOKE_REQUIRES_ROUTE",
						Severity: diagnostic.SeverityError,
						Path:     fmt.Sprintf("$.stages.%s.services.%s.smokePath", stageName, serviceName),
						Message:  fmt.Sprintf("smokePath for service %q in stage %q requires public or tailnet exposure", serviceName, stageName),
					})
				}
			}
			if forwardAuth, _ := stageService["forwardAuth"].(bool); forwardAuth {
				exposure, _ := stageService["exposure"].(string)
				if exposure != "tailnet" {
					out.Add(diagnostic.Diagnostic{
						Code:     "FORWARDAUTH_REQUIRES_TAILNET_ROUTE",
						Severity: diagnostic.SeverityError,
						Path:     fmt.Sprintf("$.stages.%s.services.%s.forwardAuth", stageName, serviceName),
						Message:  fmt.Sprintf("forwardAuth for service %q in stage %q requires tailnet exposure", serviceName, stageName),
					})
				}
			}
			writableTmpfsPaths, _ := runtime["writableTmpfsPaths"].([]any)
			if relaxed && len(writableTmpfsPaths) > 0 {
				out.Add(diagnostic.Diagnostic{
					Code:     "RELAXED_RUNTIME_HAS_TMPFS_PATHS",
					Severity: diagnostic.SeverityError,
					Path:     fmt.Sprintf("$.stages.%s.services.%s.runtime.writableTmpfsPaths", stageName, serviceName),
					Message:  fmt.Sprintf("service %q in stage %q is runtime.relaxed and cannot declare read-only-runtime tmpfs paths", serviceName, stageName),
				})
			}
		}
		requestedEngines := make(map[string]bool, len(platform))
		for engine := range platform {
			requestedEngines[engine] = true
		}
		for _, rawService := range services {
			service, _ := rawService.(map[string]any)
			dataServices, _ := service["dataServices"].([]any)
			for _, rawEngine := range dataServices {
				delete(requestedEngines, rawEngine.(string))
			}
		}
		for engine := range requestedEngines {
			out.Add(diagnostic.Diagnostic{
				Code:     "UNUSED_PLATFORM_SERVICE",
				Severity: diagnostic.SeverityError,
				Path:     fmt.Sprintf("$.stages.%s.platform.%s", stageName, engine),
				Message:  fmt.Sprintf("stage %q requests platform service %q but no service declares it in dataServices", stageName, engine),
			})
		}
	}
	return out
}

// hostnameClaim enforces single-owner routing within one document across all
// stages and services.
type hostnameClaim struct {
	byHostname map[string]string
}

func (c *hostnameClaim) add(hostname, owner, path string, out *diagnostic.List) {
	if c.byHostname == nil {
		c.byHostname = map[string]string{}
	}
	if other, exists := c.byHostname[hostname]; exists {
		out.Add(diagnostic.Diagnostic{
			Code:     "DUPLICATE_HOSTNAME",
			Severity: diagnostic.SeverityError,
			Path:     path,
			Message:  fmt.Sprintf("hostname %q is already claimed by %s of this document", hostname, other),
		})
		return
	}
	c.byHostname[hostname] = owner
}

// SortedServiceNames returns deterministic service-name ordering for callers.
func SortedServiceNames(services map[string]any) []string {
	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
