// Package diagnostic defines the stable structured diagnostic model shared by
// the human and JSON output paths.
package diagnostic

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Severity values.
const (
	SeverityError = "error"
)

// Diagnostic is one structured validation finding. The message is always a
// stable human-readable description; it never contains raw document values or
// source excerpts.
type Diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	File     string `json:"file,omitempty"`
	Document int    `json:"document,omitempty"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

// List is an ordered collection of diagnostics with deterministic ordering.
type List []Diagnostic

// Add appends one diagnostic.
func (l *List) Add(d Diagnostic) {
	*l = append(*l, d)
}

// Sort orders diagnostics deterministically by file, code, and path.
func (l List) Sort() {
	sort.SliceStable(l, func(i, j int) bool {
		if l[i].File != l[j].File {
			return l[i].File < l[j].File
		}
		if l[i].Code != l[j].Code {
			return l[i].Code < l[j].Code
		}
		return l[i].Path < l[j].Path
	})
}

// HasErrors reports whether any diagnostic is an error.
func (l List) HasErrors() bool {
	for _, d := range l {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// JSON renders the list as a stable JSON array.
func (l List) JSON() ([]byte, error) {
	return json.MarshalIndent(l, "", "  ")
}

// Human renders the list for terminal output.
func (l List) Human() string {
	var out string
	for _, d := range l {
		loc := d.File
		if d.Path != "" {
			loc = fmt.Sprintf("%s: %s", loc, d.Path)
		}
		out += fmt.Sprintf("%s: %s: %s\n", loc, d.Code, d.Message)
	}
	return out
}
