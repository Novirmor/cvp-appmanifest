# ADR 0009: Define the v1alpha1 Document Shape

- Status: Accepted
- Date: 2026-08-14

## Context

The first shape was a single Git-built container with one HTTP port and one
route per stage. Real applications in this platform have more than one service,
and some services must not be routed at all.

## Decision

Multi-service applications: each service is a prebuilt image or a Git-built
container, and every stage places the complete application. Exposure is
`public`, `tailnet`, or `internal`. Secrets are stage-level opaque references
(ADR 0005) consumed as environment values or Docker-secrets-style file mounts.

```yaml
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: example-site
services:
  web:
    source:
      repository: https://github.com/example/static-site.git
      build: { context: ., dockerfile: Dockerfile }
    httpPort: 8080
  db:
    source: { image: "postgres:17-alpine" }
    volumes:
      - { name: data, path: /var/lib/postgresql/data }
volumes:
  data: {}
stages:
  prod:
    target: { host: edge-1 }
    secrets:
      db_password:
        secretRef: <reference>
    services:
      web:
        revision: <full-sha>
        exposure: public
        hostname: app.example.com
        environment:
          NGINX_PORT: { value: "8080" }
      db:
        exposure: internal
        environment:
          POSTGRES_PASSWORD: { secret: db_password }
```

Decisions left **outside** the shared schema, as MGconsulting executor policy:
stage-name policy, production commit-SHA policy, repository host and
organization allowlist, inventory membership, DNS and exposure consistency,
secret resolution and authorization, Traefik and Compose details, build
sandboxing, reconciliation, rollback, and runtime naming.

## Consequences

`stages.<name>.target.host` is required, so a document names the node it runs
on.

**Under review:** architecture
[ADR 0012](https://github.com/MGconsulting/architecture/blob/main/docs/adr/0012-appmanifest-scope-and-placement.md)
(Proposed) would remove `target.host` in favor of executor-resolved placement,
and add a `platform` block carrying tenancy requests. Both are breaking changes
to `v1alpha1`, deliberately taken while no executor consumes the format. If
that ADR is accepted, this ADR is superseded on those two points.
