// Package corpus discovers and validates complete deployment corpora, including
// cross-document collision checks.
package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MGconsulting/appmanifest/internal/diagnostic"
	"github.com/MGconsulting/appmanifest/internal/document"
	"github.com/MGconsulting/appmanifest/internal/normalize"
	"github.com/MGconsulting/appmanifest/internal/validation"
)

// CanonicalEnvelopeAPIVersion identifies the retained corpus artifact format.
const CanonicalEnvelopeAPIVersion = "appmanifest.mgconsulting.io/corpus/v1alpha1"

// MaxFiles bounds the number of documents per corpus.
const MaxFiles = 1000

// Document is one validated corpus document.
type Document struct {
	File        string
	Decoded     map[string]any
	Canonical   map[string]any
	Diagnostics diagnostic.List
}

// CanonicalEnvelope is the portable, source-independent corpus artifact.
// Applications are ordered by logical name, not by input path.
type CanonicalEnvelope struct {
	APIVersion   string           `json:"apiVersion"`
	Applications []map[string]any `json:"applications"`
}

// Discover returns the top-level *.yaml files of dir in sorted order. .yml
// files, symlinks, special files, and paths escaping the corpus root are
// rejected.
func Discover(dir string) ([]string, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yml") {
			return nil, fmt.Errorf("%s: .yml extension is not accepted; use .yaml", filepath.Join(root, name))
		}
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		mode := info.Mode()
		if mode&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%s: symlinks are not accepted", filepath.Join(root, name))
		}
		if !mode.IsRegular() {
			return nil, fmt.Errorf("%s: special files are not accepted", filepath.Join(root, name))
		}
		files = append(files, filepath.Join(root, name))
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s: corpus contains no *.yaml files", root)
	}
	if len(files) > MaxFiles {
		return nil, fmt.Errorf("%s: corpus exceeds the %d file limit", root, MaxFiles)
	}
	return files, nil
}

// ValidateAll validates every file and runs cross-document checks. The input
// order is preserved; diagnostics are sorted deterministically.
func ValidateAll(files []string) ([]Document, diagnostic.List, error) {
	var docs []Document
	var all diagnostic.List

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, err
		}
		doc := Document{File: f}
		decoded, err := document.Decode(data)
		if err != nil {
			doc.Diagnostics.Add(diagnostic.Diagnostic{
				Code:     "YAML_DECODE",
				Severity: diagnostic.SeverityError,
				File:     f,
				Message:  err.Error(),
			})
		} else {
			doc.Decoded = decoded
			doc.Diagnostics = append(doc.Diagnostics, validation.ValidateSchema(decoded)...)
			doc.Diagnostics = append(doc.Diagnostics, validation.ValidateSemantic(decoded)...)
			// Schema/semantic diagnostics do not know their file; stamp it so
			// corpus output always locates the offending document.
			for i := range doc.Diagnostics {
				if doc.Diagnostics[i].File == "" {
					doc.Diagnostics[i].File = f
				}
			}
			if !doc.Diagnostics.HasErrors() {
				canonical, err := normalize.Canonical(decoded)
				if err != nil {
					doc.Diagnostics.Add(diagnostic.Diagnostic{
						Code:     "NORMALIZE",
						Severity: diagnostic.SeverityError,
						File:     f,
						Message:  err.Error(),
					})
				} else {
					doc.Canonical = canonical
				}
			}
		}
		all = append(all, doc.Diagnostics...)
		docs = append(docs, doc)
	}

	all = append(all, crossDocumentChecks(docs)...)
	all.Sort()
	return docs, all, nil
}

// Envelope returns canonical documents sorted by application name. It omits
// corpus source paths. Stage secret maps contain opaque references, not secret
// values, and remain available to the executor for approved resolution.
func Envelope(docs []Document) (CanonicalEnvelope, error) {
	applications := make([]map[string]any, 0, len(docs))
	for _, doc := range docs {
		if doc.Canonical == nil {
			return CanonicalEnvelope{}, fmt.Errorf("cannot create an envelope from an invalid corpus")
		}
		applications = append(applications, cloneMap(doc.Canonical))
	}
	sort.Slice(applications, func(i, j int) bool {
		return applications[i]["name"].(string) < applications[j]["name"].(string)
	})
	return CanonicalEnvelope{
		APIVersion:   CanonicalEnvelopeAPIVersion,
		Applications: applications,
	}, nil
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = clone(value)
	}
	return out
}

func clone(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = clone(item)
		}
		return out
	default:
		return value
	}
}

func crossDocumentChecks(docs []Document) diagnostic.List {
	var out diagnostic.List
	names := make(map[string]string)
	hostnames := make(map[string]string)
	for _, d := range docs {
		if d.Canonical == nil {
			continue
		}
		name, _ := d.Canonical["name"].(string)
		if name != "" {
			if other, exists := names[name]; exists {
				out.Add(diagnostic.Diagnostic{
					Code:     "DUPLICATE_PROJECT_NAME",
					Severity: diagnostic.SeverityError,
					File:     d.File,
					Path:     "$.name",
					Message:  fmt.Sprintf("project name %q is already used by %s", name, other),
				})
			} else {
				names[name] = d.File
			}
		}
		stages, _ := d.Canonical["stages"].(map[string]any)
		for stageName, rawStage := range stages {
			stage, _ := rawStage.(map[string]any)
			stageServices, _ := stage["services"].(map[string]any)
			for svcName, rawSS := range stageServices {
				ss, _ := rawSS.(map[string]any)
				hostname, _ := ss["hostname"].(string)
				if hostname == "" {
					continue
				}
				key := fmt.Sprintf("service %q of stage %q in %s", svcName, stageName, d.File)
				if other, exists := hostnames[hostname]; exists {
					out.Add(diagnostic.Diagnostic{
						Code:     "DUPLICATE_HOSTNAME",
						Severity: diagnostic.SeverityError,
						File:     d.File,
						Path:     fmt.Sprintf("$.stages.%s.services.%s.hostname", stageName, svcName),
						Message:  fmt.Sprintf("hostname %q is already claimed by %s", hostname, other),
					})
				} else {
					hostnames[hostname] = key
				}
			}
		}
	}
	return out
}
