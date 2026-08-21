# ADR 0008: Publish Reproducible Cross-Platform Artifacts

- Status: Accepted
- Date: 2026-08-14

## Context

Consumers install the CLI on operator machines and CI runners of several
architectures, and must be able to verify what they installed.

## Decision

- Initial release matrix: Linux amd64, Linux arm64, macOS amd64, macOS arm64,
  Windows amd64.
- Pure Go: `CGO_ENABLED=0`, `-trimpath`, with version, commit, and schema
  digest injected at build time.
- Publish platform archives, `SHA256SUMS`, an SBOM, provenance, and the
  immutable `v1alpha1` schema asset.

## Consequences

Every deployment can name the validator version and schema digest that accepted
it, which is what makes a validation result auditable later.
