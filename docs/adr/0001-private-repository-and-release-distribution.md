# ADR 0001: Ship appmanifest From a Private Repository by Release

- Status: Accepted
- Date: 2026-08-14

## Context

The format and its validator are consumed by `cvp-infra-ansible` and, later, by the
executor. Distribution has to work for both a local operator and CI without
publishing the schema before its secret-reference and routing models are
deliberately generic.

## Decision

- `MGconsulting/appmanifest` is **private** initially.
- Distribution is by GitHub Release from the private repository.
- Local install: `mise` GitHub backend using `gh` credentials.
- CI install: a GitHub App token scoped to exactly `appmanifest`, provided
  through the reusable lint/security workflows or a dedicated secret.
- Artifacts are checksum-verified (`SHA256SUMS`).

## Consequences

Consumers need credentials to install the CLI, so the install path is part of
every consumer's bootstrap rather than a public download. Revisit public
visibility once the secret-reference and routing models are intentionally
generic.
