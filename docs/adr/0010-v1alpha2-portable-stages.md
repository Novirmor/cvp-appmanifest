# ADR 0010: Add v1alpha2 Portable Stages

- Status: Proposed
- Date: 2026-08-30

## Context

The v1alpha1 shape pins every stage to `target.host`. It cannot state portable
placement requirements or an application's portable request for the supported
data engines. This conflicts with the executor choosing a suitable workload
node and leaves the engine request in separate registries.

## Decision

Add v1alpha2 alongside the unchanged v1alpha1 schema.

- A v1alpha2 stage has no `target` field.
- `placement.requireComponents` is optional, normalizes to `["workload"]`,
  and must include `workload` when explicitly declared.
- A stage may request `postgres`, `rabbitmq`, and `redis` through empty objects
  in `platform`.
- Services may declare `dataServices`; validation rejects a service requesting
  an engine that its stage does not request.
- The normalized corpus is a deterministic, name-sorted JSON envelope without
  source filenames or stage secret references.

## Consequences

The v1alpha1 schema and behavior remain available for existing consumers. New
consumers must dispatch by `apiVersion`, and placement, tenancy implementation,
credentials, and engine topology remain executor or platform policy.

## Verification

Unit and black-box tests cover v1alpha1 compatibility, v1alpha2 structural and
semantic validation, version-specific defaults, and the redacted corpus
envelope. `task lint` and `task test` must pass.
