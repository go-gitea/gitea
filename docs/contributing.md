---
icon: lucide/git-pull-request
---

# Contribution Guidelines

This document explains how to contribute to Teabag. Topic-specific guides live in separate files so the essentials are easy to navigate:

| Topic | Guide |
|:------|:------|
| **Setup & Requirements** | [Setup and requirements](build-setup.md) |
| **Development Workflow** | [Development workflow](development.md) |
| **Build from Source** | [Building from source](build-source.md) |
| **Testing** | [Testing guide](testing.md) |
| **Frontend Guidelines** | [Frontend development guidelines](guidelines-frontend.md) |
| **Backend Guidelines** | [Backend development guidelines](guidelines-backend.md) |
| **Refactoring** | [Refactoring guidelines](guidelines-refactoring.md) |
| **Governance & Review** | [Community governance](community-governance.md) |
| **Release Management** | [Release management](release-management.md) |

---

## AI Contribution Policy

Contributions made with the assistance of AI tools are welcome, provided contributors use them responsibly:

1. Review AI-assisted code thoroughly before opening a pull request.
2. Manually test changes and provide automated tests where feasible.
3. Only submit contributions you understand well enough to explain, defend, and revise yourself during review.
4. Disclose AI assistance clearly. When committing with AI pair programmers, include an `Assisted-by: AGENT_NAME:MODEL_VERSION` trailer.
5. Do not use automated bots or LLMs to blindly answer reviewer questions; engagement during review must be human and substantive.

---

## Issues

### Reporting Issues

Before opening a new issue:
1. Search the issue tracker to ensure the problem has not already been reported.
2. Test on the latest `main` branch if possible.
3. Provide concise reproduction steps, environment details (OS, architecture, database version), and relevant server logs.
4. For security-sensitive vulnerabilities, please email **security@teabag.local** rather than filing a public issue.

### Issue Categories

- `bug`: Unexpected behavior in frontend, backend, or CLI.
- `feature`: Proposed new functionality aligned with Teabag's lightweight homelab mission.
- `enhance`: Polish, UX tweaks, or small non-breaking improvements.
- `perf`: Measurable performance and resource footprint improvements.
- `refactor`: Structural code cleanup that does not alter behavior.

---

## Pull Request Guidelines

### Structure & Scope

- **Keep PRs focused**: Smaller PRs are reviewed and merged much faster.
- **Avoid unrelated modifications**: Keep styling cleanups or unrelated refactoring in separate PRs.
- **UI Changes**: Any change affecting the UI must include **after** screenshots in the PR description, and **before** screenshots when modifying existing interfaces. Aim for minimal PR descriptions focusing on *what* and *why*.

### Conventional Commits & PR Titles

Pull requests in Teabag are squash-merged; the PR title becomes the squash commit subject. Titles must follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```text
type(scope)!: subject
```

Allowed types:

- `feat`: User-facing feature or major addition.
- `enhance`: Minor user-facing polish or UX improvement.
- `fix`: Bug fix or security patch.
- `perf`: Performance enhancement.
- `refactor`: Code reorganization without functional changes.
- `docs`: Documentation updates.
- `test`: Adding or updating test cases.
- `build`: Build system, packaging, or dependency updates.
- `ci`: CI configuration or integration updates.
- `chore`: Auxiliary tasks and tooling updates.

A `!` before the colon denotes a breaking change (e.g. `feat(api)!: modify repository response payload`).

---

## Breaking Changes

A change is considered breaking if:
- It alters public API contract responses or endpoints.
- It removes or renames configuration keys in `app.ini`.
- It requires manual administrator intervention during an upgrade.

If a PR contains a breaking change:
1. Provide a clear rationale for why the change is necessary.
2. Include a `## :warning: BREAKING :warning:` section in the PR description detailing the user impact and mitigation steps.

---

## Review Process

- Review begins when a pull request is marked ready for review (non-draft).
- Do not rebase or force-push an active pull request unless explicitly requested; push incremental commits to keep review diffs readable.
- Maintainers will squash-merge approved pull requests according to the [Governance guide](community-governance.md).
