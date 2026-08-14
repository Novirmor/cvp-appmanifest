# deployment-spec

Authoritative deployment-document format for MGconsulting: versioned schema,
validation CLI, and conformance fixtures.

This repository owns the contract and its validator. It does **not** own
inventory, credentials, Ansible execution, DNS, or concrete deployment
definitions — those live in `infra-ansible`, `infra-tf`, and `ansible-collection`
respectively.

## Layout

- `schema/v1alpha1/deployment.schema.json` — immutable v1 schema (embedded by
  the Go package; no copies).
- `cmd/deployment-spec` — CLI entry point.
- `internal/` — document decoding, normalization, validation, corpus checks,
  and diagnostics.
- `examples/` — sanitized, generic example documents (neutral hosts and
  `example.com` domains).
- `docs/adr/` — release-blocking decisions.

## CLI

```sh
go run ./cmd/deployment-spec version
go run ./cmd/deployment-spec validate --corpus examples
go run ./cmd/deployment-spec validate --file examples/whoami.yaml --json
go run ./cmd/deployment-spec normalize --file examples/whoami.yaml
```

Exit codes: `0` valid, `1` validation errors, `2` usage or operational failure.

Corpus validation is authoritative: pre-commit, CI, and deployments must always
validate the complete corpus, never only changed files, so duplicate project
names and routed hostnames cannot be missed.

## Document contract (v1alpha1)

```yaml
apiVersion: deployment.mgconsulting.io/v1alpha1
name: app
source:
  repository: https://github.com/MGconsulting/app.git
  build: { context: ., dockerfile: Dockerfile }
container:
  httpPort: 8080
stages:
  prod:
    revision: <full-sha>
    target: { host: edge-1 }
    environment:
      KEY: { value: "plain" }
      SECRET: { secretRef: bws://<uuid> }
    route: { hostname: app.example.com, exposure: public }
```

YAML input is restricted: one document per file, no aliases/merge keys/custom
tags/timestamps, string keys only, duplicate keys rejected, and resource limits
enforced. See `docs/format.md` once published and `docs/adr/` for decisions.

## Development

```sh
mise install      # once
task lint         # pre-commit: gofmt, go test, actionlint, corpus validation
task test         # go test -race + vet
task security     # gitleaks + zizmor + govulncheck
task artifact     # reproducible release artifacts + SHA256SUMS
```

CI runs the organization `lint`, `test`, and `security` reusable workflows,
pinned by commit SHA.
