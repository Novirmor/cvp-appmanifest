// Command appmanifest validates and normalizes deployment documents.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/MGconsulting/appmanifest/internal/corpus"
	"github.com/MGconsulting/appmanifest/internal/document"
	"github.com/MGconsulting/appmanifest/internal/normalize"
	"github.com/MGconsulting/appmanifest/internal/validation"
)

// Exit codes: 0 valid, 1 validation errors, 2 usage/operational failure.
const (
	exitOK      = 0
	exitInvalid = 1
	exitOper    = 2
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return exitOper
	}
	switch args[0] {
	case "version":
		return cmdVersion()
	case "validate":
		return cmdValidate(args[1:])
	case "normalize":
		return cmdNormalize(args[1:])
	default:
		usage()
		return exitOper
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: appmanifest <validate|normalize|version> [options]")
	fmt.Fprintln(os.Stderr, "  appmanifest validate --corpus <dir> [--json]")
	fmt.Fprintln(os.Stderr, "  appmanifest validate --file <path> [--json]")
	fmt.Fprintln(os.Stderr, "  appmanifest normalize --file <path>")
	fmt.Fprintln(os.Stderr, "  appmanifest version")
}

func cmdVersion() int {
	commitInfo := commit
	if info, ok := debug.ReadBuildInfo(); ok && commitInfo == "unknown" {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				commitInfo = s.Value
				break
			}
		}
	}
	fmt.Printf("appmanifest %s (commit %s)\n", version, commitInfo)
	fmt.Printf("supported apiVersions: %v\n", validation.SupportedAPIVersions)
	return exitOK
}

func cmdValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	var (
		corpusDir = fs.String("corpus", "", "directory of *.yaml deployment documents")
		file      = fs.String("file", "", "single deployment document to validate")
		jsonOut   = fs.Bool("json", false, "emit JSON diagnostics")
	)
	if err := fs.Parse(args); err != nil {
		return exitOper
	}
	if fs.NArg() != 0 {
		usage()
		return exitOper
	}
	if (*corpusDir == "") == (*file == "") {
		usage()
		return exitOper
	}

	var files []string
	var err error
	if *corpusDir != "" {
		files, err = corpus.Discover(*corpusDir)
	} else {
		files = []string{*file}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitOper
	}

	_, diags, err := corpus.ValidateAll(files)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitOper
	}

	if *jsonOut {
		out, err := diags.JSON()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return exitOper
		}
		fmt.Println(string(out))
	} else {
		fmt.Fprint(os.Stderr, diags.Human())
	}

	if diags.HasErrors() {
		return exitInvalid
	}
	return exitOK
}

func cmdNormalize(args []string) int {
	fs := flag.NewFlagSet("normalize", flag.ContinueOnError)
	file := fs.String("file", "", "deployment document to normalize to canonical JSON")
	if err := fs.Parse(args); err != nil {
		return exitOper
	}
	if *file == "" || fs.NArg() != 0 {
		usage()
		return exitOper
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitOper
	}
	decoded, err := document.Decode(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitInvalid
	}
	diags := validation.ValidateSchema(decoded)
	diags = append(diags, validation.ValidateSemantic(decoded)...)
	diags.Sort()
	if diags.HasErrors() {
		fmt.Fprint(os.Stderr, diags.Human())
		return exitInvalid
	}
	canonical, err := normalize.Canonical(decoded)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitInvalid
	}
	out, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitOper
	}
	fmt.Println(string(out))
	return exitOK
}
