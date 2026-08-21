# ADR 0004: Validate With Embedded Draft 2020-12 Schemas

- Status: Accepted
- Date: 2026-08-14

## Context

Validation must be reproducible offline and identical in CI, on an operator's
machine, and inside the executor.

## Decision

- Library: `github.com/santhosh-tekuri/jsonschema/v6` for Draft 2020-12.
- Vocabulary limited to what the implementation needs; format assertion policy
  documented per field.
- Network `$ref` retrieval is forbidden; references are local-only.
- Schemas are immutable and embedded by Go package.

## Consequences

A schema change is a code change with a new release, which is the intent: the
schema is versioned API, not configuration.
