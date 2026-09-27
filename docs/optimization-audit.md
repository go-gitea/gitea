# Optimization audit

## Scope and method

This is a source and build-configuration audit of the current checkout. It does not
claim that a package is unused solely because a search found few references.
Go and frontend dependency reachability could not be fully resolved because
dependencies are not available locally and outbound access to `proxy.golang.org`
is blocked. The only change in this continuation removes the Snap build package
`libsqlite3-dev`; application behavior and the optional native SQLite/PAM paths
are unchanged.

The local working tree was clean at inspection. `make help` lists the repository's
build and maintenance targets. Relevant build guidance is in
[`build-setup.md`](build-setup.md), [`build-source.md`](build-source.md),
[`development.md`](development.md), [`testing.md`](testing.md),
[`guidelines-backend.md`](guidelines-backend.md), and
[`guidelines-frontend.md`](guidelines-frontend.md).

## 0. LDAP/LDAPS Removal (Teabag Fork)

**Summary**: Complete removal of LDAP/LDAPS authentication support as it is outside
Teabag's target use case.

### Components Removed

- **Authentication Backend**: `services/auth/source/ldap/` (6 files, ~1,160 lines)
  - `source.go` - LDAP source configuration
  - `source_authenticate.go` - LDAP authentication logic
  - `source_search.go` - LDAP user/group search
  - `source_sync.go` - LDAP user synchronization
  - `security_protocol.go` - LDAP/LDAPS protocol handling
  - `util.go` - LDAP utility functions
  - `assert_interface_test.go` - Interface compliance test
  - `README.md` - LDAP documentation

- **Admin CLI Commands**: `cmd/admin_auth_ldap.go` (473 lines) and `cmd/admin_auth_ldap_test.go` (1,348 lines)

- **Admin Web UI**: `templates/admin/auth/source/ldap.tmpl` (147 lines)

- **Database Migration**: `modelmigration/v1_16/v189.go` (110 lines) and `v189_test.go` (82 lines)
  - Migration "Unwrap ldap.Sources" that handled LDAP config unwrapping
  - Registration removed from `modelmigration/migrations.go`

- **Database Compatibility Placeholders**: Removed gaps in `models/auth/source.go`
  - Type 2 (LDAP) and Type 5 (DLDAP) constants marked "retained for database compatibility"
  - Type constants re-sequenced: NoType=0, Plain=1, SMTP=2, PAM=3, OAuth2=4, SSPI=5

- **Integration Tests**: `tests/integration/auth_ldap_test.go` (550 lines)

- **Frontend**: `web_src/js/features/admin/common.ts` (LDAP-related code removed)

- **Locale Strings**: Removed stale LDAP translations from 24 non-English locale files
  - Keys removed: `admin.auths.verify_group_membership`, `admin.auths.map_group_to_team`,
    `admin.auths.map_group_to_team_removal`, `admin.auths.enable_ldap_groups`,
    `admin.auths.ssh_keys_are_verified`

- **Routers/Handlers**: LDAP-related code removed from:
  - `routers/web/admin/auths.go`
  - `services/auth/sync.go`
  - `services/forms/auth_form.go`
  - `routers/api/v1/user/key.go`
  - `routers/web/user/setting/keys.go`
  - `models/user/user.go`
  - `services/asymkey/ssh_key_test.go`
  - `services/user/update_test.go`
  - `services/user/user_test.go`

### Dependencies Removed

- `github.com/go-ldap/ldap/v3 v3.4.14` (direct dependency)
- `github.com/go-asn1-ber/asn1-ber v1.5.8` (indirect, ASN.1 BER for LDAP)
- `github.com/hashicorp/go-uuid v1.0.3` (indirect, used by LDAP)
- `github.com/jcmturner/aescts/v2 v2.0.0` (indirect, Kerberos crypto for LDAP)
- `github.com/jcmturner/dnsutils/v2 v2.0.0` (indirect, Kerberos DNS for LDAP)
- `github.com/jcmturner/gofork v1.7.6` (indirect, Kerberos for LDAP)
- `github.com/jcmturner/goidentity/v6 v6.0.1` (indirect, Kerberos for LDAP)
- `github.com/jcmturner/gokrb5/v8 v8.4.4` (indirect, Kerberos for LDAP)
- `github.com/jcmturner/rpc/v2 v2.0.3` (indirect, Kerberos RPC for LDAP)
- `github.com/alexbrainman/sspi v0.0.0-20250919150558-7d374ff0d59e` (indirect, SSPI for LDAP)

### Authentication Behavior Preserved

All non-LDAP authentication mechanisms remain functional:
- Local accounts (Plain/Password)
- SMTP authentication
- PAM authentication
- OAuth2/OIDC (GitHub, GitLab, Google, OpenID Connect, etc.)
- SSPI/SPNEGO (Windows Integrated Authentication)

### Maintenance/Attack-Surface Rationale

- **Attack surface**: LDAP protocol parsing, LDAPS/TLS certificate handling, LDAP injection risks, and Kerberos/SPNEGO integration code removed
- **Maintenance burden**: ~3,500 lines of LDAP-specific code removed; no more LDAP schema changes, protocol updates, or AD/OpenLDAP compatibility testing
- **Dependencies**: 10 transitive dependencies removed, reducing supply-chain risk
- **Binary size**: Reduced by eliminating LDAP protocol handling code
- **Startup overhead**: No LDAP connection pooling or schema discovery on startup
- **Memory usage**: No LDAP connection caches or search result buffers

## 1. High-confidence removable dependencies

None established from available evidence. The manifest and source tree are not
enough to prove a dependency unused: Go build tags, generators, test-only imports,
and dynamic feature wiring must be included in reachability checks. `go list` could
not resolve module metadata in this environment, so no module removal is proposed.
`go.mod` declares 252 requirements (105 direct, 147 indirect); this is a manifest
count rather than the set linked into the main binary. Offline package enumeration
reports 94 module versions missing from the local cache.

The module manifests support reliable classification into the 105 declared direct
and 147 declared indirect requirements only. A per-module production/test/tool/
platform graph cannot be produced here: `go list -deps`, `go mod why`, and the
license inventory all need module resolution. No one of the 252 is therefore called
unused or removable in this audit. Tool package versions pinned in Makefile are
separate on-demand `go install` tools and are not requirements in `go.mod`.

## 2. High-confidence dead code

None established. Repository-wide dead-code detection was not completed: a complete
Go package graph and frontend dependency installation were unavailable. Static
search alone is insufficient evidence to delete exported, dynamically registered,
generated, or template-referenced code.

## 3. CGO elimination candidates

| Candidate | Evidence and use | Expected benefit | Risk / replacement |
|---|---|---|---|
| `github.com/mattn/go-sqlite3` (`sqlite_mattn` + `sqlite_unlock_notify`) | Its driver file has both build constraints; the Makefile only enables CGO for `sqlite_mattn` or `pam`. The default driver is `modernc.org/sqlite` under `!sqlite_mattn`. Integration test comments document a concurrent-write hang with mattn's unlock-notify path. | Keep the normal build free of C toolchains and SQLite system headers; potentially simplify release environments. | It is an opt-in alternative, so removing it breaks users who explicitly build with those tags. Do not remove without checking supported release/build workflows and compatibility expectations. Default already uses the pure-Go alternative. |
| PAM (`github.com/msteinert/pam/v2`) | `modules/auth/pam/pam.go` is selected only by `pam`; the stub is selected otherwise. Makefile sets CGO for the `pam` tag. | No CGO for builds that do not opt into PAM; already achieved by default. | Removing it breaks optional system PAM authentication and requires Linux PAM headers/libraries when enabled. Preserve as an optional feature unless product policy drops PAM. |

The Makefile sets `CGO_ENABLED=0` by default. `make generate` also runs `go generate`
with CGO disabled. The ordinary build is designed for pure Go, but this checkout
could not complete `go build` to verify it. A `CGO_ENABLED=0 go build` validation
is still required with dependencies available.

### All identified CGO-enabled paths

- The Makefile defaults to `CGO_ENABLED=0` and flips it to `1` if `TAGS` includes
  `sqlite_mattn` or `pam`. The mattn driver source additionally requires
  `sqlite_unlock_notify`; its feature is an alternate SQLite driver. The normal
  `!sqlite_mattn` driver imports `modernc.org/sqlite` and is pure Go.
- The `pam` tag selects `modules/auth/pam/pam.go` and `github.com/msteinert/pam/v2`;
  without the tag, `pam_stub.go` is selected. Snapcraft requests `libpam0g-dev`,
  `libsqlite3-dev`, and `build-essential`, and its build script uses
  `TAGS="bindata pam" make build`. Docker includes `linux-pam` runtime libraries.
- `build/generate-go-licenses.go` launches `go list` with `CGO_ENABLED=1` for its
  Linux/amd64 license inventory. This is a build-time inventory tool, not a normal
  application-binary requirement.
- `make generate-go` explicitly sets `CGO_ENABLED=0`; Makefile `build` runs frontend
  and backend, with backend using the Makefile CGO setting. Release tooling is
  documented as Go native cross compilation.
- Repository search found no Go `import "C"`, `#cgo`, `go:build cgo`, or legacy
  `+build cgo` files. The two application CGO paths are selected by explicit
  feature tags, whose CGO imports are in external packages.

The default production build is therefore configured for `CGO_ENABLED=0`. Source
inspection supports that expectation, but binary verification is blocked by missing
Go modules. CGO remains available for optional PAM and mattn SQLite features; neither
is recommended for removal.

| Classification | Component / files | CGO purpose and default status | Replacement / compatibility | Recommendation |
|---|---|---|---|---|
| Required runtime CGO | None found | No Go C imports or untagged CGO integration found. Default Makefile and release builds set CGO off. | Not applicable. | Keep default production builds CGO-free; run the actual build when modules are available. |
| Optional runtime CGO | PAM: `modules/auth/pam/pam.go`, `modules/auth/pam/pam_stub.go`, `services/auth/source/pam/*`, `modules/auth/pam/pam_test.go` | `pam` tag selects the native `msteinert/pam` adapter. Snap currently builds with `TAGS="bindata pam"`; ordinary builds select the stub. | PAM is a host authentication protocol; no equivalent pure-Go path can call the system PAM stack with the same behavior. | Retain as explicit optional feature. Snap still needs `libpam0g-dev` and a compiler. |
| Optional runtime CGO | SQLite: `models/db/driver_sqlite_mattn.go` | Requires `sqlite_mattn && sqlite_unlock_notify`; Makefile enables CGO for `sqlite_mattn`. No CI/release workflow selects these tags. | Default is pure-Go `modernc.org/sqlite`. Removing mattn would drop downstream opt-in compatibility and its unlock-notify behavior. | Retain optional path; document modernc as the default. |
| Development-only CGO mode | `build/generate-go-licenses.go` | Sets `CGO_ENABLED=1` in the child `go list` environment to inventory Linux/amd64 dependency variants; not a runtime dependency. | No replacement needed; this mode ensures license inventory includes cgo-tagged variants. | Retain; does not require a C compiler to run `go list` itself, though dependency discovery is currently blocked. |
| Test-only tagged coverage | `modules/auth/pam/pam_test.go`; `tests/integration/api_issue_test.go` | PAM tests are selected only by `pam`. The integration test mentions a mattn unlock-notify hang but is not tagged `sqlite_mattn` and does not require that driver. No dedicated mattn-driver test file was found. | Default SQLite integration coverage uses modernc. | No test-only CGO requirement identified; the tag-specific paths remain unverified locally. |
| Platform-specific native dependency | PAM external module and Snap build recipe | System PAM is a Linux/Unix facility; Snap builds PAM explicitly. Docker includes `linux-pam` runtime package because its `TAGS` argument can enable the feature. | No behavior-identical pure-Go replacement for system PAM. | Keep platform libraries where optional PAM builds need them. |

### SQLite findings

- `models/db/driver_sqlite_modernc.go` is selected whenever `sqlite_mattn` is absent;
  it registers modernc's database/sql driver. This is the default production and
  local-development path and does not require a system SQLite library.
- `models/db/driver_sqlite_mattn.go` requires both `sqlite_mattn` and
  `sqlite_unlock_notify`. The Makefile turns CGO on for the first tag. This path
  configures shared cache, busy timeout, immediate transactions, and journal mode;
  unlock-notify adds mattn-specific waiting behavior. Test comments describe a
  concurrency hang in this driver. It is absent from CI and release tag selections
  found in this repository, but downstream tag usage cannot be determined here.
- No tests were found that require the mattn tags. The default SQLite path was not
  build- or behavior-tested in this environment because the required modernc module
  version is missing from the module cache.
- Snap's build script uses `bindata pam`, so modernc is selected. Its previous
  `libsqlite3-dev` build package was unnecessary for that selection and has been
  removed. The Snap still stages the `sqlite3` CLI and retains PAM's native build
  requirements. No Snap package build could be run here.

## 4. Frontend dependency reduction candidates

No package qualifies for removal yet. `node_modules` is absent, and the lockfile
could not be evaluated with package-manager tooling or a production build. `public/assets`
contains 480 files totaling 609,163 bytes (~0.58 MiB); this is source asset size,
not a measured Vite bundle size.

Vue 3 is actively used by the component tree and is documented as the framework for
interactive pages. Removing Vue is unsupported by current evidence. Optional,
page-specific libraries (for example CodeMirror, Mermaid, Chart.js, PDFObject, and
3D viewer code) are plausible chunk-loading/usage audit targets, but their imports,
route loading, and generated chunk sizes must be measured before proposing removals.
`@vitejs/plugin-vue` appears in both dependency groups in `package.json`; that
duplicate manifest entry is a concrete cleanup candidate, but its impact is
manifest/build hygiene only, not an application bundle reduction. Confirm package
manager behavior and normalize it in a separately reviewed change.

`package.json` groups the 101 unique names as 59 runtime declarations and 43
development declarations, with `@vitejs/plugin-vue` duplicated across groups.
`pnpm-lock.yaml` records a resolved importer, but the local pnpm cannot run to verify
lock consistency or resolve installed package contents. This prevents a trustworthy
full package-by-package production/dev/test classification and bundle attribution.
The source tree has 43 files importing Vue or calling `createApp`: 23 `.vue`
components, 18 non-test TypeScript feature/module files, and 2 tests. Active pages
include dashboard, repository actions, issue/pull request, diff/file trees, code
frequency, heatmap, contributors, recent commits, and workflow graph. `@vitejs/plugin-vue`
is imported by `tools/shared.ts` for the Vite build; `vue-tsc`, `eslint-plugin-vue`,
and `eslint-plugin-vue-scoped-css` are declared development tools. Their package
tasks could not be exercised without pnpm. A text scan found 31 of 101 unique npm
declarations without literal occurrences under source/config files, largely types
and lint/test tools; that is not evidence of unused packages because tool use can be
indirect through config loading.

## 5. Build-system simplification candidates

The Makefile exposes separate install, generation, frontend/backend build, lint,
test, and release workflows. Documentation confirms `make build` combines frontend
and backend builds; `bindata` embeds assets for distribution, while development
builds use dynamic assets. No target can be declared redundant from the help output
alone. Review target call sites in CI/release workflows before changing them.

Generators include embedded bindata for templates, options, migration schemas, and
public assets, plus generated charset data. They support self-contained release
builds and should not be removed without checking build tags and release packaging.

The Go tree has 3,136 `.go` files in 386 directories containing Go files, including
1,049 `_test.go` files. Frontend source has 412 files under `web_src`. The Go import
graph and generated frontend chunk sizes remain unavailable because Go module and
pnpm dependency steps could not run.

## 6. Runtime optimization candidates

No runtime memory, CPU, or startup optimization is supported by measurements yet.
The baseline document records which measurements are pending. Optional search,
cache, queue, mail, and storage integrations are configuration-driven capabilities;
their packages may be linked even when disabled, but removal would reduce supported
deployments. First measure a representative production build and workload, then
consider build-tagging optional integrations only if startup/RSS/binary evidence
justifies the added build matrix.

## 7. Risky changes requiring further investigation

- Replacing the default `modernc.org/sqlite` driver: mattn can be faster or smaller
  in some environments but requires CGO and system toolchains. The source documents
  modernc as about 2 MiB larger than mattn and notes similar CI times; those are
  project comments, not fresh measurements. Driver behavior differs for lock waits.
- Removing Git, crypto, image, archive/compression, markup, or database libraries:
  these implement core repository, authentication, avatar, attachment, package,
  and multi-database functionality. Full import and feature tracing is needed.
- Replacing crypto/network/template/logging packages with standard-library code:
  API semantics, security review, protocol edge cases, and maintenance cost outweigh
  theoretical package count reductions without a specific measured target.
- Pruning checked-in assets or generated files: templates, runtime asset references,
  bindata generation, and release packaging may consume them indirectly.
- Removing any optional integration or feature flag: configuration compatibility
  and plugin-like registration patterns must be inspected across code, docs, and tests.

For dependency-level follow-up, generate the module graph and package import usage
with the full module cache available, distinguish main-binary imports from tools and
tests, and inspect build-tag variants. For the frontend, use the lockfile-aware
package manager and Vite manifest/chunk report, then map each package to source
imports and page entrypoints.

## 8. Things that SHOULD NOT be removed

- Vue 3 based on current evidence: it is actively used for interactive components.
- The pure-Go SQLite driver: it enables the documented default CGO-free SQLite
  build and is central to local development and common single-node deployments.
- The CGO-free default build path: it reduces release toolchain and system-library
  requirements.
- Optional PAM and mattn SQLite paths without a compatibility decision: both are
  explicitly build-tagged capabilities, not accidental default dependencies.
- Database drivers for MySQL, PostgreSQL, and MSSQL, or optional integrations, until
  supported-platform/product policy explicitly drops them.
- Embedded templates, options, migration schemas, and public assets used by
  self-contained builds.

## 9. Code Optimizations from LDAP Removal

### Dead Code Removal
- **Database migration v189**: Removed "Unwrap ldap.Sources" migration (110 lines) that was only relevant for upgrading databases with LDAP sources
- **Database compatibility placeholders**: Removed gaps in auth source type constants (types 2 and 5), simplifying the type system
- **Stale locale strings**: Removed 5 LDAP-related keys from 24 locale files (~120 lines total)

### Simplified Abstractions
- **Auth source type constants**: Re-sequenced from sparse (0, 1, _, 3, 4, _, 6, 7) to dense (0, 1, 2, 3, 4, 5), eliminating "retained for database compatibility" comments
- **Auth source Names map**: Automatically stays in sync with constants since it uses constant keys

### Dependency Reduction
- **10 transitive dependencies removed**: Direct removal of `go-ldap` and its Kerberos/ASN.1/SSPI dependency chain
- **CGO risk reduced**: Removed SSPI dependency that could require CGO on Windows

### Fixed Issues
- **Missing import in models/asymkey/ssh_key.go**: Added missing `gitea.dev/models/auth` import that was implicitly satisfied by LDAP code paths
- **Missing authService in cmd package**: Implemented `cmd/admin_auth_service.go` with proper interface for OAuth2/SMTP admin commands

### Verification
- `CGO_ENABLED=0 go build` succeeds
- `go vet ./...` passes
- Core tests pass: `models/auth`, `services/auth`, `routers/web/admin`, `models/asymkey`, `cmd`
- Binary builds and runs: `gitea version development built with go1.27.1`

## Local measurement limits

Go 1.27.1, Node v26.7.0, pnpm, GCC/CC, Make, `sqlite3`, and openSUSE `zypper` are
present. The 176 MiB Go module cache lacks 94 versions requested during offline
package enumeration. DNS to `proxy.golang.org` failed with `operation not permitted`,
blocking module downloads. The Go build cache (229 MiB) is outside writable paths.
The pnpm executable cannot open its database under the read-only home/cache paths;
the existing 352 MiB store cannot be used for a lockfile install in this session.
No system packages were installed: they do not supply pinned Go/npm dependencies,
and installation would not solve the cache/network restriction.

RPM inspection confirms GCC 16, GCC C++, Make 4.4.1, Go 1.27.1, pnpm 11.9.0, and
the SQLite CLI are installed. Node 26.7.0 runs from a non-RPM installation. `pam-devel`
and `sqlite3-devel` are absent, and pkg-config cannot find either library. They are
not needed for the default CGO-free build; the Snap's isolated Ubuntu build declares
PAM headers and compiler packages for its explicit PAM tag. `snapcraft`, Docker, and
Podman are absent, preventing package/container build verification.

`public/assets` contains 480 files totaling 609,163 bytes. `web_src/js` totals
1,052,437 bytes and `web_src/css` 352,651 bytes. No generated Vite manifest/chunks
or Gitea executable/build artifact exists in this checkout. See
[`optimization-baseline.md`](optimization-baseline.md) for measurements and the
reproduction plan.

## Changes in this continuation

Removed `libsqlite3-dev` from `snap/snapcraft.yaml` build packages. The Snap script
selects modernc SQLite (`bindata pam` tags), so the mattn SQLite C library is not
compiled there. This removes one declared native build dependency without changing
the packaged application or optional PAM support. Snapcraft was unavailable, so
the packaging build remains unverified. No Go/npm modules, CGO feature paths, or
application code were removed.
