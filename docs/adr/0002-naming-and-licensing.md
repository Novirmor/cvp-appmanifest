# ADR 0002: Name and License the Project

- Status: Accepted
- Date: 2026-08-14

## Context

The repository name, module path, and binary name are load-bearing: they appear
in import paths, release assets, and every consumer's tooling pin.

## Decision

- Repository name: `appmanifest`.
- Go module path: `github.com/MGconsulting/appmanifest`.
- CLI binary name: `appmanifest`.
- License: GPL-3.0-or-later, matching the MGconsulting collection ecosystem.

## Consequences

Changing any of these later is a breaking change for consumers, so they are
fixed before the first release rather than after.
