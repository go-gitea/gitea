---
icon: lucide/compass
---

# Design Philosophy & Architecture

Teabag is a lightweight, non-enterprise Git forge forked from Gitea. It is intentionally tailored for individuals, homelabs, small teams, and open-source communities who want a reliable, modern, self-hosted Git platform without enterprise baggage.

---

## Why Teabag?

Modern software projects often suffer from feature creep. In upstream enterprise Git platforms, substantial development effort goes toward supporting legacy corporate infrastructure, complex directory federation, and heavy embedded execution runners.

For individuals and small organizations, these features introduce:

- **Expanded Attack Surface**: Legacy protocols (such as LDAP, PAM, and SSPI) and complex enterprise integrations require ongoing security scrutiny and often expose high-severity vulnerabilities.
- **Resource Footprint**: Monolithic services and embedded container runner engines demand excessive CPU, RAM, and background maintenance.
- **Codebase Complexity**: Multi-tiered enterprise abstraction layers complicate local debugging, custom extensions, and contribution workflows.

Teabag takes a clear stance: **strip the bloat, streamline the core, and prioritize the self-hoster experience.**

---

## Core Principles

### 1. Homelab & Self-Hosting First

Teabag is built to be fast and resource-efficient on everything from a Raspberry Pi or low-power mini PC to dedicated cloud VMs:

- **Pure-Go SQLite by Default**: Uses modern pure-Go SQLite drivers (`modernc.org/sqlite`), eliminating the requirement for CGO or native SQLite C library development packages.
- **Single-Binary Simplicity**: Distributes as a self-contained binary with embedded assets via the `bindata` build tag.
- **Broad Architecture Support**: Runs natively on Linux, macOS, Windows, FreeBSD, and OpenBSD across `x86_64`, `ARM64`, `RISC-V 64`, and `PowerPC`.

### 2. Streamlined Authentication (Non-Enterprise)

Teabag removes corporate-only authentication backends that add maintenance and security overhead without benefiting homelabs or community servers:

| Authentication Method | Upstream Gitea | Teabag | Rationale |
|:----------------------|:--------------|:-------|:----------|
| **Local / Database**  | Supported     | Supported | Primary method for self-hosters and small teams |
| **OAuth2 / OIDC**     | Supported     | Supported | Modern identity federation (GitHub, GitLab, Authentik, Keycloak, etc.) |
| **SMTP / Email Auth** | Supported     | Supported | Standard verification and basic user onboarding |
| **Two-Factor (TOTP / WebAuthn)** | Supported | Supported | Essential account security for public and private instances |
| **LDAP / LDAPS**      | Supported     | **Removed** | Legacy corporate directory; heavy maintenance and security attack surface |
| **PAM**               | Supported     | **Removed** | OS-level Pluggable Authentication; requires CGO and system-level privileges |
| **SSPI**              | Supported     | **Removed** | Windows domain-specific NTLM/Kerberos authentication |

Removing LDAP, PAM, and SSPI deleted thousands of lines of legacy code, unused database migrations, and complex admin UI panels while maintaining full compatibility with modern OAuth2/OIDC identity providers.

### 3. Generic External CI Integration

Rather than embedding a complex, resource-heavy CI runner daemon inside the forge, Teabag adopts an **external CI integration model**:

- **Generic Webhooks**: Delivers standard payloads on `push`, `pull_request`, `status`, and `release` events.
- **Commit Status API**: External CI runners (such as [Woodpecker](https://woodpecker-ci.org/), [Drone](https://drone.io/), [CircleCI](https://circleci.com/), or [Buildkite](https://buildkite.com/)) report build and test states directly to commits and pull requests via standard REST endpoints (`POST /repos/{owner}/{repo}/statuses/{sha}`).
- **Decoupled Architecture**: Keeps the forge responsive and stable; a runaway test suite or container build in CI cannot starve the web UI or Git daemon of CPU or memory.

### 4. ActivityPub Federation Groundwork

Instead of closed silos or monolithic federation protocols, Teabag is laying minimal, modular groundwork for the **ActivityPub** protocol and **ForgeFed** vocabulary:

- Standardized actor and object representations for users, repositories, and issues.
- Strict security boundaries (inbound signature verification, payload size limits, rate limiting, and configurable allow/block lists).
- Modular design where federation consumes existing repository and issue interfaces rather than duplicating storage layers.

---

## Technical Standards

Contributions to Teabag adhere to modern, lean engineering standards:

- **Go Idioms**: Take advantage of modern Go language features (generics, standard library slices/maps). Run `make fmt` and `make lint-go`.
- **Frontend Quality**: Modern TypeScript with strict typing. Modern Vue single-file components. Utility-first Tailwind CSS (`tw-*`) styling, systematically replacing legacy Fomantic UI overrides.
- **Verifiable Optimizations**: Changes affecting size or dependencies are tracked against measured baselines (`docs/optimization-baseline.md`).
- **Fast, Focused Testing**: Sub-second unit tests and fast SQLite-based integration tests.
