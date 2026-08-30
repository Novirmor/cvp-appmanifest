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
- `schema/v1alpha2/appmanifest.schema.json` — portable-stage schema (embedded
  by the Go package; no copies).
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
go run ./cmd/appmanifest normalize --corpus examples > corpus.json
```

Exit codes: `0` valid, `1` validation errors, `2` usage or operational failure.

Corpus validation is authoritative: pre-commit, CI, and deployments must always
validate the complete corpus, never only changed files, so duplicate project
names and routed hostnames cannot be missed.

## Document Contracts

`v1alpha1` remains supported unchanged. It requires
`stages.<name>.target.host` and does not support portable placement, tenancy
requests, or service data-service declarations.

### v1alpha1

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
- **exposure** — `public` (reachable from the internet through the platform
  router), `tailnet` (reachable only from the private network), or `internal`
  (not routed; reachable only from sibling services of the same application). A
  hostname is required iff the exposure is routed; `httpPort` is required on
  any service routed in some stage.
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

### v1alpha2

`v1alpha2` makes a stage portable: it has no `target` and therefore never names
a host. The executor resolves optional placement constraints against its own
placement catalog.

```yaml
apiVersion: appmanifest.mgconsulting.io/v1alpha2
name: example-app
services:
  web:
    source:
      image: example/web:1
    dataServices: [postgres, redis]
stages:
  prod:
    placement:
      requireComponents: [workload]
    platform:
      postgres: {}
      redis: {}
    services:
      web:
        exposure: internal
```

- `placement.requireComponents` is optional and defaults to `[workload]` in
  canonical output. If stated, it must include `workload`; additional component
  constraints are executor-resolved.
- `platform` is optional. Its only requests in this slice are `postgres`,
  `rabbitmq`, and `redis`. PostgreSQL and RabbitMQ requests are empty objects.
  Redis requires `passwordSecret`, the name of a declared stage secret whose
  opaque reference resolves to the manually provisioned ACL password. Engine
  topology and credential values remain outside the format.
- A service may declare `dataServices` from `postgres`, `rabbitmq`, and `redis`.
  It grants only the corresponding stage-network attachment. A matching
  `stage.platform` request provisions a tenant identity; operational services
  can attach to an engine without creating an unused tenant.
- `stages.<stage>.services.<service>.runtime` is the explicit runtime contract:
  active services declare a numeric non-root `user`, or a reviewed
  `relaxed: true` exception. Non-relaxed services receive a read-only root
  filesystem and can declare `writableTmpfsPaths`. `forwardAuth` is tailnet
  route-only; `smokePath` is a routed HTTP 2xx check; and `meshTcp` declares a
  safe container port behind an infrastructure-owned TCP entrypoint.
- `lifecycle` defaults to `active`. Set it to `retiring` to remove a stage's
  runtime while retaining tenant data. Set it to `purge` only after retirement
  evidence exists to authorize tenant data removal; deleting a stage from the
  corpus is never a deletion authorization.

## Corpus Artifact

`normalize --corpus <dir>` first validates the complete corpus, including
cross-document name and hostname collisions. It then writes an indented JSON
envelope with `apiVersion: appmanifest.mgconsulting.io/corpus/v1alpha1` and an
`applications` array sorted by logical application name. JSON object keys are
also emitted deterministically by the Go encoder.

The artifact contains canonical documents and applied defaults, never source
filenames, paths, or secret values. It retains stage `secrets` maps because
their entries are opaque references that the executor must resolve only after
approval. Invalid corpora produce diagnostics on stderr and no JSON artifact.

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
