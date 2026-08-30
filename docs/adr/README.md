# Architecture Decision Records

Decisions about the manifest format, its validator, and how both are
distributed. Recorded 2026-08-14 as the Phase 1 release-blocking set and
split into individual records here.

Cross-repository and trust-boundary decisions live in the
[`architecture`](https://github.com/MGconsulting/architecture) repository
instead; architecture ADR 0012 (accepted 2026-08-21) supersedes ADR 0009 below
on node placement and adds a tenancy block to the format.

| ADR | Status | Decision |
| --- | --- | --- |
| [0001](0001-private-repository-and-release-distribution.md) | Accepted | Ship from a private repository by checksum-verified GitHub Release. |
| [0002](0002-naming-and-licensing.md) | Accepted | Fix the repository name, module path, binary name, and GPL-3.0-or-later license. |
| [0003](0003-yaml-parsing-contract.md) | Accepted | Constrain the accepted YAML subset and enforce it at the node level. |
| [0004](0004-json-schema-implementation.md) | Accepted | Validate with embedded, offline-only Draft 2020-12 schemas. |
| [0005](0005-opaque-stage-level-secret-references.md) | Accepted | Carry secrets as opaque stage-level references; the store is executor policy. |
| [0006](0006-canonical-json-as-retained-artifact.md) | Accepted | Retain canonical JSON as the deployment artifact; never reparse source YAML. |
| [0007](0007-corpus-is-authoritative-desired-state.md) | Accepted | Treat the complete corpus as authoritative desired state; absence means removal. |
| [0008](0008-release-platforms-and-artifacts.md) | Accepted | Publish reproducible cross-platform artifacts with checksums, SBOM, and provenance. |
| [0009](0009-v1alpha1-document-shape.md) | Accepted; placement and tenancy scope superseded by architecture ADR 0012 | Define the multi-service v1alpha1 document shape. |
| [0010](0010-v1alpha2-portable-stages.md) | Proposed | Add portable stages and portable data-engine requests in v1alpha2. |

## Template

```markdown
# ADR NNNN: Title

- Status: Proposed | Accepted | Superseded
- Date: YYYY-MM-DD

## Context

## Decision

## Consequences

## Verification
```
