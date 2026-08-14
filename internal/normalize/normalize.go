// Package normalize applies documented defaults exactly once and produces a
// deterministic canonical value model for an application manifest.
package normalize

import "fmt"

// Canonical returns a new value model with defaults applied. It never mutates
// the input. Defaults for v1alpha1: repository-build contexts default to "." and
// "Dockerfile"; missing service volume lists, stage secret maps, stage-service
// environments, and mounts become empty collections.
func Canonical(doc map[string]any) (map[string]any, error) {
	out := copyMap(doc)

	services, err := objectAt(out, "services")
	if err != nil {
		return nil, err
	}
	for name, raw := range services {
		svc, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("services.%s: expected mapping", name)
		}
		source, ok := svc["source"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("services.%s.source: expected mapping", name)
		}
		if _, isGit := source["repository"]; isGit {
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
		}
		if _, ok := svc["volumes"]; !ok {
			svc["volumes"] = []any{}
		}
	}

	stages, err := objectAt(out, "stages")
	if err != nil {
		return nil, err
	}
	for stageName, raw := range stages {
		stage, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("stages.%s: expected mapping", stageName)
		}
		if _, ok := stage["secrets"]; !ok {
			stage["secrets"] = map[string]any{}
		}
		stageServices, ok := stage["services"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("stages.%s.services: expected mapping", stageName)
		}
		for svcName, rawSS := range stageServices {
			ss, ok := rawSS.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("stages.%s.services.%s: expected mapping", stageName, svcName)
			}
			if _, ok := ss["environment"]; !ok {
				ss["environment"] = map[string]any{}
			}
			if _, ok := ss["mounts"]; !ok {
				ss["mounts"] = []any{}
			}
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
