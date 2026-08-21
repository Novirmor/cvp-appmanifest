# ADR 0006: Retain Canonical JSON as the Deployment Artifact

- Status: Accepted
- Date: 2026-08-14

## Context

If the executor reparses source YAML at deploy time, the deployed thing and the
validated thing can differ.

## Decision

- Canonical JSON is a **retained deployment artifact** on the target under
  `/opt/projects/<...>/deployment.json`, root-owned, for reproducibility and
  audit.
- `infra-ansible` loads canonical JSON with `from_json`; source YAML is never
  reparsed at deploy time.
- Normalization applies defaults exactly once and is the sole authority for
  defaults.

## Consequences

Defaults live in exactly one implementation. What is on the host is what was
validated, and it can be diffed after the fact.
