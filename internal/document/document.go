// Package document decodes a deployment document from YAML into a
// JSON-compatible value model, enforcing the accepted YAML subset described in
// the implementation plan.
package document

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Error describes one YAML decoding failure with a document-relative path.
type Error struct {
	Path string
	Msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Path, e.Msg)
}

// MaxDepth bounds nesting to prevent pathological inputs.
const MaxDepth = 50

// Decode parses one YAML document and converts it to a JSON-compatible value
// model. Multi-document input, aliases, merge keys, custom tags, timestamps,
// non-string mapping keys, and duplicate keys are rejected.
func Decode(data []byte) (map[string]any, error) {
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	var node yaml.Node
	if err := dec.Decode(&node); err != nil {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err == nil {
		return nil, errors.New("yaml: expected exactly one document")
	} else if err != io.EOF {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	if node.Kind != yaml.DocumentNode || len(node.Content) != 1 {
		return nil, errors.New("yaml: expected exactly one document")
	}
	root := node.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, &Error{Path: "$", Msg: "document root must be a mapping"}
	}
	value, err := convert(root, "$", 0)
	if err != nil {
		return nil, err
	}
	mapping, ok := value.(map[string]any)
	if !ok {
		return nil, &Error{Path: "$", Msg: "document root must be a mapping"}
	}
	return mapping, nil
}

func convert(node *yaml.Node, path string, depth int) (any, error) {
	if depth > MaxDepth {
		return nil, &Error{Path: path, Msg: "maximum nesting depth exceeded"}
	}
	switch node.Kind {
	case yaml.MappingNode:
		return convertMapping(node, path, depth)
	case yaml.SequenceNode:
		return convertSequence(node, path, depth)
	case yaml.ScalarNode:
		return convertScalar(node, path)
	default:
		return nil, &Error{Path: path, Msg: fmt.Sprintf("unsupported node kind %d", node.Kind)}
	}
}

func convertMapping(node *yaml.Node, path string, depth int) (map[string]any, error) {
	out := make(map[string]any, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]
		if keyNode.Kind != yaml.ScalarNode || keyNode.Tag != "!!str" {
			return nil, &Error{Path: path, Msg: "mapping keys must be strings"}
		}
		if keyNode.Value == "" {
			return nil, &Error{Path: path, Msg: "empty mapping key"}
		}
		if _, exists := out[keyNode.Value]; exists {
			return nil, &Error{Path: join(path, keyNode.Value), Msg: "duplicate key"}
		}
		val, err := convert(valNode, join(path, keyNode.Value), depth+1)
		if err != nil {
			return nil, err
		}
		out[keyNode.Value] = val
	}
	return out, nil
}

func convertSequence(node *yaml.Node, path string, depth int) ([]any, error) {
	out := make([]any, 0, len(node.Content))
	for i, item := range node.Content {
		val, err := convert(item, fmt.Sprintf("%s[%d]", path, i), depth+1)
		if err != nil {
			return nil, err
		}
		out = append(out, val)
	}
	return out, nil
}

func convertScalar(node *yaml.Node, path string) (any, error) {
	switch node.Tag {
	case "!!str":
		return node.Value, nil
	case "!!int":
		v, err := strconv.ParseInt(node.Value, 0, 64)
		if err != nil {
			return nil, &Error{Path: path, Msg: fmt.Sprintf("invalid integer %q", node.Value)}
		}
		return v, nil
	case "!!float":
		v, err := strconv.ParseFloat(node.Value, 64)
		if err != nil {
			return nil, &Error{Path: path, Msg: fmt.Sprintf("invalid float %q", node.Value)}
		}
		return v, nil
	case "!!bool":
		v, err := strconv.ParseBool(node.Value)
		if err != nil {
			return nil, &Error{Path: path, Msg: fmt.Sprintf("invalid boolean %q", node.Value)}
		}
		return v, nil
	case "!!null":
		return nil, nil
	case "!!timestamp":
		return nil, &Error{Path: path, Msg: "timestamp values are not supported; quote the value as a string"}
	case "!!merge":
		return nil, &Error{Path: path, Msg: "merge keys are not supported"}
	case "!!binary":
		return nil, &Error{Path: path, Msg: "binary values are not supported"}
	default:
		return nil, &Error{Path: path, Msg: fmt.Sprintf("custom tag %q is not supported", node.Tag)}
	}
}

func join(parent, child string) string {
	if parent == "$" {
		return "$." + child
	}
	return parent + "." + child
}

// IsDuplicateKeyError reports whether err is a duplicate-key error.
func IsDuplicateKeyError(err error) bool {
	var de *Error
	return errors.As(err, &de) && strings.Contains(de.Msg, "duplicate key")
}
