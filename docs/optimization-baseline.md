# Optimization baseline

Recorded 2026-09-27 in the repository environment. These are observations from
this checkout, not estimates. No optimization was applied before recording them.

## Available measurements

| Measurement | Result | Method / scope |
|---|---:|---|
| Go version | `go1.27.1 linux/amd64` | `go version` |
| Go source | 3,136 `.go` files in 386 directories containing Go files; 1,049 `_test.go` files | Recursive repository file/directory counts. Directory count is not a resolved Go package graph. |
| Declared Go requirements | 252 entries: 105 direct and 147 marked indirect | Parsed both `go.mod` require blocks; not the resolved build graph. |
| `go.sum` entries | 866 lines | Line count; includes module and `go.mod` checksums. |
| Declared npm dependencies | 59 runtime entries; 43 development entries; 101 unique package names | Parsed `package.json`; one name is duplicated across groups. |
| Frontend source | 412 files under `web_src`; `web_src/js` is 1,052,437 bytes and `web_src/css` is 352,651 bytes | Recursive file count and `du -sb`; source size, not bundle size. |
| Vue usage | 43 TS/Vue files import Vue or call `createApp`: 23 Vue SFCs, 18 non-test TS feature/module files, and 2 TS tests | Recursive literal search for `from 'vue'` and `createApp(`. Shows active usage; does not measure bundled runtime size. |
| Checked-in `public/assets` | 480 files; 609,163 bytes (~0.58 MiB) | Recursive file count and sum of file sizes. No generated `.vite` manifest or built JS/CSS chunks were present. |
| CGO policy in Makefile | Off by default; enabled for `sqlite_mattn` or `pam` tags | Makefile inspection. |
| Local build tools | Go 1.27.1, Node v26.7.0, pnpm 11.9.0 executable, GCC 16, Make 4.4.1, and `sqlite3` CLI present | `go version`, `node --version`, `rpm -q`, and `command -v`. System is openSUSE Tumbleweed; `zypper` exists. `nodejs` RPM is absent although Node runs from another installation. |
| Native development headers | `pam-devel` and `sqlite3-devel` not installed; `pkg-config` cannot find `pam` or `sqlite3` | `rpm -q` and `pkg-config --modversion`. These are not required for the default CGO-free build; Snap's PAM build gets its own Ubuntu build packages. |
| Container/package builders | `snapcraft`, `docker`, and `podman` not found in PATH | `command -v`; Snap and Docker packaging cannot be executed locally. |
| Snap build packages | 3 declared after this change, down from 4 | `snap/snapcraft.yaml`; removed `libsqlite3-dev` because the Snap build tags select modernc SQLite. Build essentials remain for tagged PAM. |

## Not measured

| Measurement | Status and reason |
|---|---|
| Go binary size | No baseline binary exists. Build was not run because module resolution attempted network access and DNS is blocked. |
| Stripped binary size | Same; no binary available to strip. |
| Go build time | Not measured; a successful, dependency-complete build is required. |
| Frontend bundle size | Not measured; `node_modules` is absent and pnpm could not open its local database. `public/assets` size above is not a substitute. |
| Resolved dependency count / main binary graph | Not measured. `GOCACHE=/tmp/teabag-gocache GOPROXY=off go list -deps ./...` fails: 94 requested modules lack cached metadata/source, followed by errors trying to create module-cache paths under the read-only `/home/italiatroller/go/pkg/mod`. The manifest count above is not a graph count. |
| Actual CGO-linked packages | Not measured; package graph resolution failed. Source inspection shows tag-gated PAM and mattn SQLite files and a default modernc SQLite driver. |
| Runtime RSS | Not measured; no binary and no controlled server/database workload were available. |
| Startup time | Not measured; no binary and no controlled configuration/data set were available. |

## Tooling and cache availability

The Go module cache occupies 176 MiB and the Go build cache 229 MiB, but the
module cache does not contain the versions required by this checkout. An offline
package listing announces 94 missing module versions. The environment previously
reported DNS resolution failure for `proxy.golang.org` (`operation not permitted`),
so Go modules cannot be fetched in this session. The system has `zypper`, but no
package-manager installation was attempted: OS packages cannot provide this
project's pinned Go modules or pnpm lockfile package versions, and adding unrelated
system packages would not solve the blocker.

The pnpm executable exists, and its v11 store occupies 352 MiB. It exits before
reporting its version with `[ERROR] unable to open database file`; the pnpm store
and home/cache paths are outside the writable workspace. `pnpm` is installed as
RPM 11.9.0, while `package.json` requests pnpm 12.4.2; even the installed binary
cannot start to provide a lockfile compatibility check. Node itself is available,
but this prevents lockfile installation and frontend builds. `npm cache verify`
also failed with `EROFS` while trying to update the read-only npm cache. The
workspace itself and `/tmp` are writable.

## Changes and before/after

The Snap build no longer declares `libsqlite3-dev`. Its build script uses
`TAGS="bindata pam"`, which selects modernc SQLite rather than the CGO mattn
driver. The declared build-package list is 4 entries before and 3 after. This is a
reduction in Snap build-system dependencies only; no binary-size or build-time
benefit was measured, and the Snap package itself was not built. PAM remains enabled
in the Snap and retains its headers and compiler dependency. The optional mattn
driver remains in the Go build graph for compatibility with explicit tagged builds.

## Verification attempts

- `CGO_ENABLED=0 go build -o /tmp/teabag-cgo0 gitea.dev` was attempted with
  `GOPROXY=off` and a writable `/tmp` Go build cache. It failed before compilation
  because required module versions are absent and Go could not create download
  metadata under the read-only module cache.
- `CGO_ENABLED=0 go test ./models/db` failed for the same module-cache reason,
  including missing modernc SQLite and XORM modules. SQLite behavior is therefore
  unverified in this checkout.
- `ruby -e 'require "yaml"; YAML.load_file("snap/snapcraft.yaml")'` passed after the
  Snap package-list edit. Python YAML tooling was not installed, so no new Python
  package was added.
- Snapcraft, Docker, and Podman are absent; no Snap/container build was run. PAM
  feature tests and the optional mattn build were not run.

## Reproduction plan

With Go modules cached (or permitted module access), capture the default production
build and `CGO_ENABLED=0 go build` under the same Go version, host, tags, and clean
cache state. Record elapsed build time and `stat` sizes for normal and stripped
binaries. Install the exact pnpm version from `packageManager`, run the production
frontend target, and report compressed and uncompressed Vite entry/chunk sizes from
the emitted manifest. For RSS and startup, use a fixed configuration, empty test
repository/database, fixed Go binary, and repeated cold/warm launches; report the
measurement tool and aggregation. Compare only like-for-like runs.

## After LDAP Removal (2026-09-27)

### Measurements Captured

| Measurement | Result | Method / scope |
|---|---:|---|
| Go binary size | 145 MiB | `CGO_ENABLED=0 go build -o /tmp/gitea .` then `ls -lh` |
| Stripped binary size | Not measured | `strip` not run |
| Go build time | ~30s | Observed from `go build` completion |
| Declared Go requirements | 250 entries: 103 direct and 147 indirect | `go mod graph` parsing after `go mod tidy` |
| `go.sum` entries | 846 lines | Line count; 20 lines removed |
| Resolved dependency count | 626 modules | `go list -m all \| wc -l` after `go mod tidy` |
| Go source lines | 498,527 total | `find . -name "*.go" \| xargs wc -l` |
| Removed Go source lines | ~4,583 lines | `git diff --stat` shows 4,583 deletions, 72 insertions |
| CGO policy | Unchanged (off by default) | Makefile unchanged |
| `go vet ./...` | Passes | No output |
| Core tests | Pass | `models/auth`, `services/auth`, `routers/web/admin`, `models/asymkey`, `cmd` |

### Changes Summary

- **Removed LDAP authentication completely**: 61 files changed, ~4,583 lines deleted, 72 lines inserted
- **Dependencies removed**: 10 transitive dependencies (go-ldap, go-asn1-ber, Kerberos stack, SSPI)
- **Database migration removed**: v189 "Unwrap ldap.Sources" no longer needed
- **Auth source types simplified**: Dense numbering 0-5 instead of sparse 0,1,_,3,4,_,6,7
- **Locale cleanup**: 5 LDAP keys removed from 24 non-English locale files
- **Binary verification**: `CGO_ENABLED=0` build succeeds, binary runs and shows version
