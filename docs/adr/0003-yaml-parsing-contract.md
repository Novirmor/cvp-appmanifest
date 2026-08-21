# ADR 0003: Constrain the Accepted YAML Subset

- Status: Accepted
- Date: 2026-08-14

## Context

YAML accepts far more than a deployment document needs, and the surplus is
where ambiguity and parser attacks live. A manifest that means different things
to different parsers is not a contract.

## Decision

- Library: `gopkg.in/yaml.v3`, pinned, used only for low-level decoding into a
  node tree — never for semantic struct decoding.
- Accepted subset, enforced at the node level:
  - One document per regular file; multi-document input is rejected.
  - UTF-8 only; string mapping keys.
  - Duplicate keys rejected.
  - Aliases, anchors, merge keys, and custom tags rejected.
  - Timestamp coercion disabled; scalars must be JSON-compatible
    (object, array, string, number, bool, null).
  - Ambiguous scalars (`ON`, `YES`, leading-zero numbers) must be quoted for a
    string intent.
- Resource limits enforced: file size, nesting depth, node count, scalar size,
  input count, diagnostic count.

## Consequences

Documents that other YAML tools accept may be rejected here, deliberately.
Decoding to a node tree rather than to structs is what makes the subset
enforceable and diagnostics locatable.

## Verification

`internal/document` enforces nesting depth (`MaxDepth`). The remaining resource
limits — file size, node count, scalar size, input count, diagnostic count —
are **not yet implemented**; this gap is tracked as architecture finding F-042.
