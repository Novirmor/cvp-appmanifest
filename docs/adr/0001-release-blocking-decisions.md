# Deployment Spec — Release-Blocking Decisions (Phase 1)

Recorded 2026-08-14. These decisions unblock repository creation and the
`v1alpha1` implementation. Each becomes an ADR in
`appmanifest/docs/adr/` once the repository exists.

## ADR-001: Repository visibility and distribution

- **Decision:** `MGconsulting/appmanifest` is **private** initially.
- Distribution is by GitHub Release from the private repository.
- Local install: `mise` GitHub backend using `gh` credentials.
- CI install: a GitHub App token scoped to exactly `appmanifest`, provided
  through the reusable lint/security workflows or a dedicated secret.
- Artifacts are checksum-verified (`SHA256SUMS`).
- Revisit public visibility after the secret-reference and routing model are
  intentionally generic.

## ADR-002: Naming and licensing

- Repository name: `appmanifest`.
- Go module path: `github.com/MGconsulting/appmanifest`.
- CLI binary name: `appmanifest`.
- License: GPL-3.0-or-later (matches the MGconsulting collection ecosystem).

## ADR-003: YAML parsing contract

- Library: `gopkg.in/yaml.v3`, pinned, used only for low-level decoding into a
  node tree — never for semantic struct decoding.
- Accepted subset (enforced at the node level):
  - One document per regular file; multi-document input is rejected.
  - UTF-8 only; string mapping keys.
  - Duplicate keys rejected.
  - Aliases, anchors, merge keys, and custom tags rejected.
  - Timestamp coercion disabled; scalars must be JSON-compatible
    (object/array/string/number/bool/null).
  - Ambiguous scalars (`ON`, `YES`, leading-zero numbers) must be quoted for a
    string intent.
- Resource limits enforced: file size, nesting depth, node count, scalar size,
  input count, diagnostic count.

## ADR-004: JSON Schema implementation

- Library: `github.com/santhosh-tekuri/jsonschema/v6` (Draft 2020-12 support).
- Vocabulary limited to what the implementation needs; format assertion policy
  documented per field.
- Network `$ref` retrieval forbidden; references are local-only.
- Schemas are immutable and embedded by Go package.

## ADR-005: Secrets in `v1alpha1`

- **Decision:** stage-level `secrets` are **included** in the `v1alpha1`
  schema using **opaque references**. The reference syntax and the secret
  store are executor policy: `infra-ansible` defines, validates, and resolves
  them (e.g. as Bitwarden secret ids) **before** the cutover.
- The shared schema constrains references only to safe single-line
  identifiers; it names no store and no scheme.
- Until resolution + redaction tests are green, `infra-ansible` refuses any
  deployment whose selected stage declares an unresolved secret.
- Canonical JSON contains references, never resolved values.

## ADR-006: Canonical JSON

- **Decision:** canonical JSON is a **retained deployment artifact** on the
  target under `/opt/projects/<...>/deployment.json` (root-owned), used for
  reproducibility and audit.
- `infra-ansible` loads canonical JSON with `from_json`; source YAML is never
  reparsed at deploy time.
- Normalization applies defaults exactly once and is the sole authority for
  defaults.

## ADR-007: Lifecycle semantics

- **Decision:** the complete corpus is **authoritative desired state**.
- Absence means removal: a removed stage/project must be undeployed and its
  orphaned Compose project / Traefik router cleaned up.
- Stage rename = new identity + removal of the old instance.
- Logical identity is the `(project name, stage name)` tuple. Collision-proof
  runtime naming is an executor concern (e.g. `name-stage-<shorthash>`).

## ADR-008: Platforms and artifacts

- Initial release matrix:
  - Linux amd64, Linux arm64
  - macOS amd64, macOS arm64
  - Windows amd64
- Pure-Go `CGO_ENABLED=0`, `-trimpath`, injected version/commit/schema digest.
- Publish: platform archives, `SHA256SUMS`, SBOM, provenance, and the immutable
  `v1alpha1` schema asset.

## ADR-009: v1alpha1 document shape

Single Git-built container, one HTTP port, one route per stage instance.
Namespaced fields (no longer flat):

```yaml
apiVersion: appmanifest.mgconsulting.io/v1alpha1
name: example-site
source:
  repository: https://github.com/MGconsulting/example-static-site.git
  build: { context: ., dockerfile: Dockerfile }
container:
  httpPort: 8080
stages:
  prod:
    revision: <full-sha>
    target: { host: larry243 }
    environment:
      NGINX_PORT: { value: "8080" }
      API_KEY:   { secretRef: <reference> }
    route: { hostname: example.mgconsulting.io, exposure: public }
```

Decisions left **outside** the shared schema (MGconsulting/executor policy):
stage-name policy, production commit-SHA policy, repo host/organization
allowlist, inventory membership, DNS/exposure consistency, secret resolution &
authorization, Traefik/Compose details, build sandboxing, reconciliation,
rollback, and runtime naming.
