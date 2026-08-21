# ADR 0007: Treat the Complete Corpus as Authoritative Desired State

- Status: Accepted
- Date: 2026-08-14

## Context

A format that only describes additions cannot express removal, which is where
orphaned Compose projects and stale routers come from.

## Decision

- The complete corpus is **authoritative desired state**.
- Absence means removal: a removed stage or project must be undeployed and its
  orphaned Compose project and Traefik router cleaned up.
- A stage rename is a new identity plus removal of the old instance.
- Logical identity is the `(project name, stage name)` tuple. Collision-proof
  runtime naming is an executor concern, for example `name-stage-<shorthash>`.

## Consequences

Deleting a stage from the corpus is a destructive operation, so the executor
needs a safety workflow around it rather than a silent apply. See
`infra-ansible/playbooks/offboard.yml`, which retains volumes past a deadline
and destroys them only on explicit request.
