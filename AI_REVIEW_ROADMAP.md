# AI Code Review — CodeRabbit Feature Parity Plan

**Goal**: Transform the current inline-review foundation into a fully-featured
CodeRabbit-equivalent built into Gitea.

**Legend**:  P0=P0 (core), P1=P1 (important), P2=P2 (nice-to-have)

---

## Phase 0: Foundation ✓ (Done)

| Feature | Status |
|---|---|
| Multi-file batching | Done |
| Configurable system prompt | Done |
| Path exclusion (globs) | Done |
| Inline code comments | Done |
| Review summary posting | Done |
| Async worker queue | Done |
| Event triggers (open/update) | Done |
| Duplicate commit cache | Done |
| OpenAI-compatible provider | Done |
| Severity levels (critical/warning/info) | Done |
| Language detection (36+ langs) | Done |
| Patch truncation | Done |

---

## Phase 1: Review Quality (P0)

### 1.1 — Walkthrough & Architectural Summary
- **What**: AI generates a high-level walkthrough of changes, grouped by
  concern (e.g., "Backend API changes", "Frontend components", "Database
  migrations") with an architecture diagram (ASCII/Mermaid).
- **How**: Expand `ReviewResponse` struct with `Walkthrough` field; update
  prompt to request structured walkthrough. Render walkthrough as collapsible
  section in the review body.
- **Files**: `provider.go`, `openai.go`, `reviewer.go`
- **Effort**: Small
- **Deps**: None

### 1.2 — Review Ordering
- **What**: Files reviewed in a logical order (import dependencies first,
  then consumers) instead of alphabetical.
- **How**: Sort files by dependency order (detect imports) before passing to
  `buildReviewPrompt`.
- **Files**: `reviewer.go`, `diff.go`
- **Effort**: Small
- **Deps**: None

### 1.3 — "No Issues" Quiet Mode
- **What**: When AI finds zero issues, post nothing instead of an empty review.
- **How**: Already implemented (skips when `len(comments)==0 && summary==""`).
- **Effort**: None (done)

---

## Phase 2: Configuration (P0)

### 2.1 — Per-Repo YAML Config
- **What**: Repo owners place `.gitea/ai-review.yaml` in the default branch to
  override global settings per-repo.
- **Config shape**:
  ```yaml
  enabled: true
  path_instructions:
    "security/*.go": "Be extra strict about auth and SQL injection"
    "frontend/**/*.tsx": "Check React best practices, hooks deps"
  exclude_paths:
    - "vendor/**"
    - "*.generated.*"
  system_prompt: "..."
  ```
- **How**: Load file from repo via `gitRepo.GetFileContent()` on each review;
  merge with global settings.
- **Files**: New `config.go`, `reviewer.go`, `setting/ai_review.go`
- **Effort**: Medium
- **Deps**: None

### 2.2 — Path-Based Instructions
- **What**: Different review instructions per directory/file glob, e.g., "be
  strict about SQL in `dal/`", "ignore style in `testdata/`".
- **How**: Pass path instructions to AI via system prompt or user message.
- **Files**: `openai.go`, new `config.go`
- **Effort**: Small (if done after 2.1)
- **Deps**: Phase 2.1

### 2.3 — Custom Checks (Pre-Merge Gates)
- **What**: User defines natural-language checks that must pass — e.g.,
  "Verify all new functions have unit tests" or "No TODO comments in
  production code".
- **How**: Each check is sent to the AI as a separate prompt or as part of
  the review prompt. Checks that fail are reported; optionally block merge.
- **Files**: New `checks.go`, `reviewer.go`, config
- **Effort**: Medium
- **Deps**: Phase 2.1

---

## Phase 3: Interaction (P0)

### 3.1 — Chat Bot on PR
- **What**: Users reply to the AI review comment to ask questions ("Why did
  you flag this?"), request re-review, or give feedback.
- **How**:
  - Register a webhook/listener for `IssueComment` events on PRs.
  - Parse replies to the AI review comment (detect by comment ID or marker).
  - Send conversation history + diff to AI and post response as a reply.
  - Persist conversation per PR (in-memory or DB) so the AI has context.
- **Files**: New `chat.go`, `notifier.go` (extend for comment events), queue
- **Effort**: Large
- **Deps**: None

### 3.2 — Learnings / Feedback Loop
- **What**: When a user corrects the AI ("This is not a bug, it's
  intentional"), store the correction and apply it to future reviews.
- **How**:
  - Detect correction patterns in chat replies (e.g., "Ignore this", "False
    positive").
  - Store learnings per repo in a DB table or file.
  - Inject relevant learnings into the system prompt for subsequent reviews.
- **Files**: New `learning.go`, `chat.go`, config
- **Effort**: Medium
- **Deps**: Phase 3.1

### 3.3 — "Fix with AI"
- **What**: Each inline comment includes a suggested fix diff. User clicks a
  button (or we provide a command) to apply it as a new commit.
- **How**:
  - Extend `ReviewComment` with `SuggestedFix *SuggestedFix` containing
    old/new code blocks.
  - Create a commit on the PR branch using Gitea's git API.
  - UI: render a "Apply fix" link/button next to the comment.
- **Files**: `provider.go`, `openai.go`, new `fix.go`, `reviewer.go`
- **Effort**: Large
- **Deps**: None

### 3.4 — Re-run / Dismiss Buttons
- **What**: PR comment buttons to re-run the review or dismiss specific
  findings (stored as user feedback).
- **How**: Render buttons in review body; handle via webhook/comment events.
  Dismissed findings go into a "dismissed" list (in-memory or DB) and are
  filtered from future reviews.
- **Files**: `reviewer.go`, new `feedback.go`, `notifier.go`
- **Effort**: Medium
- **Deps**: Phase 3.1 (for the webhook wiring)

---

## Phase 4: Context & Integration (P1)

### 4.1 — Codebase Awareness (Codegraph)
- **What**: Provide AI with contextual understanding of cross-file
  dependencies (e.g., "function X is called by Y and Z, which are in other
  files").
- **How**:
  - Simple: extract import/export graphs using AST parsing (Go-specific
    `go/parser`, generic regex for others).
  - Send relevant dependency context in the review prompt.
  - Advanced: build a persistent codegraph index per repo.
- **Files**: New `codegraph.go`, `openai.go`
- **Effort**: Large
- **Deps**: None

### 4.2 — Linter Integration
- **What**: Run existing linters (golint, eslint, ruff, etc.) alongside AI
  review, merge results into one output, and let the AI filter out false
  positives.
- **How**:
  - Run linter commands on the changed files (using Git blob content).
  - Parse linter output and feed it to the AI as context ("Here's what the
    linter found — ignore false positives and report only real issues").
  - Deduplicate: don't report linter findings as AI findings.
- **Files**: New `linter.go`, `openai.go`, `reviewer.go`
- **Effort**: Large
- **Deps**: None

### 4.3 — Linked Issues Context
- **What**: Pull context from linked Jira/Linear/GitHub issues referenced in
  the PR description.
- **How**: Parse issue references (e.g., `Fixes PROJ-123`), fetch issue
  details via API (if configured), include in the review prompt.
- **Files**: New `issues_context.go`, `openai.go`, config
- **Effort**: Medium
- **Deps**: None

### 4.4 — Web Query
- **What**: AI can fetch documentation for dependencies (e.g., "Look up the
  current React Hooks API").
- **How**:
  - Provide AI with a tool/function-calling capability to fetch URLs.
  - Or: use a web search API (e.g., Tavily, Bing) as a pre-processing step.
- **Files**: `openai.go`, new `webquery.go`
- **Effort**: Medium
- **Deps**: Phase 4.1 (function calling support in provider)

---

## Phase 5: Productivity Tools (P1)

### 5.1 — Unit Test Generation
- **What**: AI detects untested functions and generates test code as a
  suggested commit.
- **How**:
  - Prompt AI to identify functions without test coverage.
  - Request test code generation in the review.
  - Provide test code as a "Fix with AI" suggestion (Phase 3.3).
- **Files**: `openai.go` (prompt extension), `fix.go`
- **Effort**: Medium
- **Deps**: Phase 3.3

### 5.2 — Docstring Generation
- **What**: AI detects missing docstrings on public APIs and generates them.
- **How**:
  - Prompt AI to identify undocumented public functions/types.
  - Generate docstrings as per-file suggested changes.
- **Files**: `openai.go` (prompt extension), `fix.go`
- **Effort**: Small
- **Deps**: Phase 3.3 (for applying)

### 5.3 — Reports (Standups, Sprint Reviews)
- **What**: Periodic summary of merged PRs — daily standup, sprint review.
- **How**:
  - Aggregation endpoint that queries merged PRs in a time range.
  - Send PR list to AI for summarization.
  - Render as Markdown in a discussion/comment.
- **Files**: New `reports.go`, routes
- **Effort**: Medium
- **Deps**: None

---

## Phase 6: Platform (P2)

### 6.1 — CLI Mode
- **What**: `gitea ai-review` command to review local uncommitted changes or
  a specific diff.
- **How**: Read local diff, invoke `ReviewCode`, print results to stdout.
- **Files**: New `cmd/aireview/`
- **Effort**: Small
- **Deps**: None

### 6.2 — Review Status Badge
- **What**: PR page shows a badge: "AI Review: Pending / In Progress /
  Complete / Issues Found".
- **How**:
  - Track review state in a DB field or cache.
  - Expose via API; render in templates.
- **Files**: `reviewer.go`, templates, API
- **Effort**: Small
- **Deps**: None

---

## Dependency Graph

```
Phase 0 ───────────────────────────────────────────────────► (done)
Phase 1 ──► 1.1 ──► 1.2 ──► 1.3 (done)
Phase 2 ──► 2.1 ──► 2.2 ──► 2.3
Phase 3 ──► 3.1 ──► 3.2 ──► 3.3 ──► 3.4
                 │          │
                 ▼          ▼
Phase 4 ──► 4.1 ──► 4.2 ──► 4.3 ──► 4.4
Phase 5 ───────────► 5.1 ──► 5.2 ──► 5.3
Phase 6 ──► 6.1 ──► 6.2
```

Arrows mean "depends on". Phases can be worked in parallel unless arrows
cross. Within a phase, items should be done top-to-bottom.

---

## Suggested Ordering

| Order | Phase | Item | Value | Effort | ROI |
|---|---|---|---|---|---|
| 1 | Phase 1 | Walkthrough & summary | High | Small | ★★★★★ |
| 2 | Phase 2 | Per-repo YAML config | High | Medium | ★★★★☆ |
| 3 | Phase 3 | Chat bot on PR | High | Large | ★★★★☆ |
| 4 | Phase 2 | Path-based instructions | High | Small | ★★★★☆ |
| 5 | Phase 3 | "Fix with AI" | High | Large | ★★★☆☆ |
| 6 | Phase 1 | Review ordering | Medium | Small | ★★★☆☆ |
| 7 | Phase 3 | Learnings/feedback | High | Medium | ★★★☆☆ |
| 8 | Phase 5 | Docstring generation | Medium | Small | ★★★☆☆ |
| 9 | Phase 6 | Status badge | Medium | Small | ★★★☆☆ |
| 10 | Phase 3 | Re-run/dismiss buttons | Medium | Medium | ★★☆☆☆ |
| 11 | Phase 4 | Codegraph/awareness | High | Large | ★★☆☆☆ |
| 12 | Phase 2 | Custom checks | Medium | Medium | ★★☆☆☆ |
| 13 | Phase 5 | Unit test gen | Medium | Medium | ★★☆☆☆ |
| 14 | Phase 4 | Linter integration | Medium | Large | ★★☆☆☆ |
| 15 | Phase 4 | Linked issues | Low | Medium | ★☆☆☆☆ |
| 16 | Phase 4 | Web query | Low | Medium | ★☆☆☆☆ |
| 17 | Phase 5 | Reports | Low | Medium | ★☆☆☆☆ |
| 18 | Phase 6 | CLI mode | Low | Small | ★☆☆☆☆ |
