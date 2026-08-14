# appmanifest

Authoritative application manifest format for MGconsulting: versioned schema,
validation CLI, and conformance fixtures.

This repository owns the contract and its validator. It does **not** own
inventory, credentials, Ansible execution, DNS, or concrete deployment
definitions — those live in `infra-ansible`, `infra-tf`, and `ansible-collection`
respectively.

## Layout

- `schema/v1alpha1/appmanifest.schema.json` — immutable v1 schema (embedded by
  the Go package; no copies).
- `cmd/appmanifest` — CLI entry point.
- `internal/` — document decoding, normalization, validation, corpus checks,
  and diagnostics.
- `examples/` — sanitized, generic example manifests (neutral hosts and
  `example.com` domains), including platform base manifests for postgres,
  redis, and rabbitmq.
- `docs/adr/` — release-blocking decisions.

## CLI

```sh
go run ./cmd/appmanifest version
go run ./cmd/appmanifest validate --corpus examples
go run ./cmd/appmanifest validate --file examples/postgres.yaml --json
go run ./cmd/appmanifest normalize --file examples/whoami.yaml
```

Exit codes: `0` valid, `1` validation errors, `2` usage or operational failure.

Corpus validation is authoritative: pre-commit, CI, and deployments must always
validate the complete corpus, never only changed files, so duplicate project
names and routed hostnames cannot be missed.

## Document contract (v1alpha1)

An application is one or more services — prebuilt images or Git-built
containers — placed per stage. Example: postgres + pgadmin.

```yaml
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: postgres
services:
  postgres:
    source:
      image: postgres:17-alpine
    volumes:
      - name: data
        path: /var/lib/postgresql/data
  pgadmin:
    source:
      image: dpage/pgadmin:9
    httpPort: 80
volumes:
  data: {}
stages:
  prod:
    target:
      host: edge-1
    secrets:
      postgres_password:
        secretRef: <reference>
    services:
      postgres:
        exposure: internal
        mounts:
          - secret: postgres_password
        environment:
          POSTGRES_PASSWORD_FILE:
            value: /run/secrets/postgres_password
      pgadmin:
        exposure: tailnet
        hostname: pgadmin.example.com
        environment:
          PGADMIN_DEFAULT_PASSWORD:
            secret: pgadmin_password
```

### Semantics

- **services** — the application's containers. `source` is either a prebuilt
  `image` (tag or sha256 digest) or a Git `repository` with an optional `build`.
- **stages** — independently deployed instances. Every stage must place every
  service (complete desired state; no partial applications).
- **revision** — required per stage for repository-built services; forbidden
  for image services.
- **exposure** — `public` (routed on the public entrypoint), `tailnet` (routed
  on the tailnet entrypoint), or `internal` (no route; reachable only from
  sibling services of the same application). A hostname is required iff the
  exposure is routed; `httpPort` is required on any service routed in some
  stage.
- **secrets** — stage-level material referenced opaquely and resolved by the
  controller. The reference syntax (e.g. a Bitwarden secret id) is executor
  policy defined in `infra-ansible`, not part of this format. Consumed two
  ways: as environment values (`environment.KEY.secret`) and as
  Docker-secrets-style files mounted at `/run/secrets/<name>` (`mounts`).
  Values never appear in manifests.
- **volumes** — named persistent volumes attached to services at absolute
  container paths.
- **Environment** — explicit per stage service; there is no inheritance.

YAML input is restricted: one document per file, no aliases/merge keys/custom
tags/timestamps, string keys only, duplicate keys rejected, and resource limits
enforced. See `docs/adr/` for decisions.

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
