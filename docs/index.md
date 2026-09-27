# Welcome to Teabag

**Teabag** is a lightweight, non-enterprise Git forge built for self-hosting.

It provides the things you expect from a Git forge—repositories, issues, pull requests, code review, and releases—without trying to become an entire enterprise collaboration suite.

> **Less baggage. More Git.**

---

## What is Teabag?

Teabag started from Gitea's codebase, but it is being deliberately shaped into its own project.

The goal isn't to add every possible enterprise feature.

The goal is to keep the core Git-forge experience useful, maintainable, and lightweight.

That means removing complexity that isn't part of Teabag's target use case while keeping the features that make a Git forge useful.

### The Teabag approach

* **Self-hosting first** — Designed for people running their own Git infrastructure.
* **Lightweight by design** — Prefer deletion and simplification over layers of unnecessary complexity.
* **CGO-free** — The project targets `CGO_ENABLED=0` builds.
* **SQLite-friendly** — SQLite remains a simple default without requiring a separate database server.
* **External CI** — Let dedicated CI systems handle builds and tests instead of turning the forge into a CI platform.
* **Open integrations** — Use standard webhooks, commit statuses, and APIs where possible.
* **Federation groundwork** — ActivityPub and ForgeFed support can grow without making federation mandatory for everyone.
* **No enterprise theater** — Teabag doesn't need a dozen layers of enterprise configuration just to host a Git repository.

---

## Why Teabag?

Large Git-forge projects can accumulate a lot of functionality over time.

Some of that functionality is useful. Some of it exists for compatibility with environments Teabag isn't targeting.

Teabag takes a different approach:

> **If a feature isn't necessary for the project's target use case, it doesn't automatically belong in the core.**

For example, Teabag has removed authentication layers such as **PAM** and **SSPI**. These aren't being claimed to be useless everywhere; they're simply outside the authentication model Teabag is targeting.

The same philosophy applies throughout the codebase:

```text
Measure
  ↓
Understand
  ↓
Remove unnecessary complexity
  ↓
Test
  ↓
Keep what actually matters
```

The objective is not to make Teabag the biggest Git forge.

It's to make it a Git forge that is easy to understand, deploy, and maintain.

---

## Key Features

### Git hosting

Host repositories, browse source code, manage branches, review changes, and collaborate through pull requests.

### Issues and code review

Track bugs and tasks and review contributions without requiring a separate collaboration platform.

### Releases

Publish releases and associated artifacts alongside the repositories they belong to.

### External CI

Teabag exposes standard webhook and commit-status interfaces so CI can remain a separate concern.

This makes it possible to connect systems such as:

* Woodpecker
* Drone
* CircleCI
* Buildkite
* Gitea Runner
* Forgejo Runner

The exact integration depends on the CI system and its supported interfaces.

### Federation groundwork

Teabag contains groundwork for ActivityPub and ForgeFed-oriented federation.

The implementation is intentionally modular and does not require federation to run a normal Teabag installation.

---

## Quick Start

### Pre-built binary

Download a Teabag release for your operating system and architecture.

Then:

```bash
chmod +x teabag
./teabag web
```

Open:

```text
http://localhost:3000
```

and complete the initial setup.

For more detailed installation information, see [Prerequisites & Setup](build-setup.md).

### Build from source

If you prefer building Teabag yourself:

```bash
git clone https://github.com/italiatroller-1990/Teabag.git
cd Teabag
CGO_ENABLED=0 go build ./...
```

See [Build from Source](build-source.md) for the complete development build process.

---

## Documentation

| Section                       | What you'll find                                       |
| ----------------------------- | ------------------------------------------------------ |
| **Philosophy & Architecture** | Why Teabag exists and how the project is structured    |
| **Hosting & Setup**           | Installation and deployment information                |
| **External CI**               | Webhooks, commit statuses, and external CI integration |
| **Federation**                | ActivityPub and ForgeFed groundwork                    |
| **Development**               | Development workflow and project conventions           |
| **Testing**                   | Test strategy and verification                         |
| **Optimization**              | Dependency, backend, and frontend audits               |
| **Community**                 | Contributing and project governance                    |

---

## Optimization

Teabag treats optimization as an engineering problem rather than a marketing number.

The project maintains audits covering:

* backend dependencies
* authentication components
* CGO usage
* frontend dependencies
* Vue components
* Vite configuration
* CSS
* lazy loading
* bundle size

The rule is simple:

> **Measure first. Delete second.**

A smaller codebase is useful when it is smaller because unnecessary complexity was removed—not because functionality was arbitrarily sacrificed.

See the [Optimization Baseline](optimization-baseline.md) and the [Frontend Optimization Audit](frontend-optimization-audit.md).

---

## Development Philosophy

Teabag favors:

1. **Simple code over unnecessary abstraction**
2. **Deletion over replacement when functionality is unnecessary**
3. **Standard interfaces over proprietary integrations**
4. **Measured optimization over guesswork**
5. **Maintainability over cleverness**
6. **Compatibility where it matters**
7. **A focused feature set over feature accumulation**

This doesn't mean every feature needs to be removed.

It means every piece of complexity should have a reason to exist.

---

## Why the name?

It's Teabag.

It's lightweight.

And, theoretically, that makes it capable of metaphorically **t-bagging bloated Git servers**.

🫖

---

## Contributing

Teabag is open source and welcomes contributions.

Before making a large change, read the development and refactoring guidelines to understand the project's approach to simplicity and compatibility.

Start with the [Contributing Guidelines](contributing.md).

---

## License

Teabag retains the applicable licensing and attribution requirements of its upstream code and dependencies.

See the repository's license and notice files for the complete legal information.
