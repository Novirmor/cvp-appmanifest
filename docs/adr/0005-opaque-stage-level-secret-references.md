# ADR 0005: Carry Secrets as Opaque Stage-Level References

- Status: Accepted
- Date: 2026-08-14

## Context

The schema must let an application say it needs a secret without naming a
secret store, or the format is bound to Bitwarden forever.

## Decision

- Stage-level `secrets` are **included** in `v1alpha1` as **opaque
  references**. The reference syntax and the secret store are executor policy:
  `cvp-infra-ansible` defines, validates, and resolves them — for example as
  Bitwarden secret ids — before the cutover.
- The shared schema constrains references only to safe single-line identifiers;
  it names no store and no scheme.
- Until resolution and redaction tests are green, `cvp-infra-ansible` refuses any
  deployment whose selected stage declares an unresolved secret.
- Canonical JSON contains references, never resolved values.

## Consequences

The format stays store-agnostic and safe to share. The obligation moves to the
executor, which must fail closed on an unresolved reference.

## Verification

`cvp-infra-ansible/playbooks/deploy.yml` validates every declared reference and
fails on an empty resolution before contacting a target host, satisfying the
fail-closed requirement (architecture finding F-005, resolved).
