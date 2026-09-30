# Refactoring guidelines

This document covers expectations for refactoring work. For the general workflow see
[CONTRIBUTING.md](../CONTRIBUTING.md).

## Background

Gitea is a large, long-lived project. Over time the codebase has accumulated
outdated mechanisms, mixed frameworks, and legacy code that can cause bugs or slow
down new features. Refactoring keeps the codebase maintainable, but it needs to be
done carefully so it improves things without introducing regressions.

## Writing a refactoring PR

- Be forward-looking: address the root cause, not just the immediate symptom.
- Aim to reduce ambiguity and conflicts and to improve maintainability.
- Explain the rationale in the PR description: why the refactor is necessary, how it
  resolves the legacy problem, and its advantages and disadvantages.
- Keep the scope tight: preserve existing behavior where feasible and avoid bundling
  unrelated changes.
- Break large refactors into intermediate steps across multiple PRs so each one is
  easy to review.
- Include tests that verify the behavior stays correct.
- Prefer scheduling non-bugfix refactoring early in a milestone, so any issues
  surface well before a release.
- If there is disagreement about a refactor, escalate to the Technical Oversight
  Committee (TOC) for a decision.

## Reviewing and merging

- Keep refactoring PRs short-lived (typically no more than 7 days) with quick review
  cycles, and merge them promptly so they do not block on unrelated work.
- A non-author core member may approve and merge a refactoring PR after 7 days if the
  TOC has raised no objection.
- Accept imperfect intermediate implementations as long as the final result improves
  the codebase.
- A temporary regression caused by a necessary refactor is acceptable if it is fixed
  promptly afterwards.
