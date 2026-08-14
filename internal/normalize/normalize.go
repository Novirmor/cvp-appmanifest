// Package normalize applies documented defaults exactly once and produces a
// deterministic canonical value model for a deployment document.
package normalize

import "fmt"

// Canonical returns a new value model with defaults applied. It never mutates
// the input. Defaults for v1alpha1: source.build.context and
// source.build.dockerfile default to "." and "Dockerfile"; a missing stage
// environment is an empty map.
func Canonical(doc map[string]any) (map[string]any, error) {
	out := copyMap(doc)

	source, err := objectAt(out, "source")
	if err != nil {
		return nil, err
	}
	build, ok := source["build"].(map[string]any)
	if !ok {
		build = map[string]any{}
		source["build"] = build
	}
	if _, ok := build["context"]; !ok {
		build["context"] = "."
	}
	if _, ok := build["dockerfile"]; !ok {
		build["dockerfile"] = "Dockerfile"
	}

	stages, err := objectAt(out, "stages")
	if err != nil {
		return nil, err
	}
	for name, raw := range stages {
		stage, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("stages.%s: expected mapping", name)
		}
		if _, ok := stage["environment"]; !ok {
			stage["environment"] = map[string]any{}
		}
	}
	return out, nil
}

func objectAt(doc map[string]any, key string) (map[string]any, error) {
	raw, ok := doc[key]
	if !ok {
		return nil, fmt.Errorf("%s: missing", key)
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: expected mapping", key)
	}
	return obj, nil
}

func copyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = deepCopy(v)
	}
	return out
}

func deepCopy(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return copyMap(t)
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = deepCopy(item)
		}
		return out
	default:
		return v
	}
}
