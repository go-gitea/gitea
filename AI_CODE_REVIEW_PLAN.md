# AI Code Review for Gitea — Master Plan

## Goal
Add a built-in AI code review feature to Gitea that automatically reviews pull requests using any OpenAI-compatible API (OpenRouter, OpenAI, Anthropic via proxy, etc.), posting inline review comments and a summary.

---

## Phase 1: Configuration & Provider Interface

### 1.1 Config (`modules/setting/ai_review.go`)
```go
package setting

type AIRreview struct {
    Enabled          bool
    Provider         string // "openrouter", "openai", or custom
    APIURL           string // OpenAI-compatible base URL
    APIToken         string
    Model            string // e.g. "openai/gpt-4o", "anthropic/claude-3.5-sonnet"
    MaxTokens        int
    Temperature      float64
    TriggerOnOpen    bool
    TriggerOnUpdate  bool
    MaxPatchSize     int    // max chars per diff chunk
    SystemPrompt     string // optional override
}

var AIRreview = &AIRreview{
    Enabled:     false,
    Provider:    "openrouter",
    APIURL:      "https://openrouter.ai/api/v1",
    Model:       "openai/gpt-4o",
    MaxTokens:   4096,
    Temperature: 0.3,
    MaxPatchSize: 80000,
}
```

Register in `modules/setting/setting.go` → `LoadSettings()` reads `[ai_review]` INI section.

### 1.2 Provider Interface (`services/aireview/provider.go`)
Define a generic interface:

```go
type Provider interface {
    ReviewCode(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error)
    Name() string
}

type ReviewRequest struct {
    Diff         string
    FilePath     string
    CommitSHA    string
    PRTitle      string
    PRDescription string
}

type ReviewComment struct {
    File    string
    Line    int
    Body    string
    Severity string // "critical", "warning", "info"
}

type ReviewResponse struct {
    Summary  string
    Comments []ReviewComment
}
```

### 1.3 OpenAI-Compatible Provider (`services/aireview/openai.go`)
Implement the provider using the OpenAI Chat Completions API format (compatible with OpenRouter, OpenAI, Together AI, etc.).

- Uses `modules/httplib` or standard `net/http` client
- Sends diff as context with a system prompt instructing the AI to review code
- Parses structured JSON from the response (use function calling / response_format for reliability)
- Handles chunking of large diffs

### 1.4 Provider Registry (`services/aireview/registry.go`)
Simple registry to map config provider name → Provider implementation. Start with `"openrouter"` and `"openai"` (both use the same OpenAI-compatible provider, different defaults).

---

## Phase 2: Triggering AI Reviews

### 2.1 Notifier Hook (`services/aireview/notifier.go`)
Create a `NullNotifier`-based notifier that listens to:

| Event | Method | When to trigger |
|-------|--------|----------------|
| PR opened | `PullRequestOpened()` | `TriggerOnOpen` |
| PR updated (new commits) | `PullRequestSynchronized()` | `TriggerOnUpdate` |
| Review requested | `PullRequestReviewRequest()` | Optional |

Register in `services/notify/notify.go` via `init()` and `RegisterNotifier()`.

### 2.2 Async Queue (`services/aireview/queue.go`)
Use Gitea's queue system (`modules/queue`) to process AI reviews asynchronously:

```go
type AIRreviewTask struct {
    PRID     int64
    DoerID   int64
    Event    string // "opened", "synchronized"
}
```

- The notifier pushes a task to a `WorkerPoolQueue[AIRreviewTask]`
- Worker handler calls `services/aireview/reviewer.go` to do the actual work
- Configurable queue settings in `[queue.ai_review]` (defaults: channel queue, 10 workers)

---

## Phase 3: Core Review Logic

### 3.1 PR Diff Fetcher (`services/aireview/diff.go`)
Fetch the diff for a pull request:
- Use `git diff` commands via `gitrepo` package
- Split by file
- Respect `MaxPatchSize` — chunk if too large
- Skip generated/lock files (node_modules, package-lock, go.sum, etc.)

### 3.2 AI Review Runner (`services/aireview/reviewer.go`)
Orchestrates the full review:

1. Fetch PR info (title, description, diff)
2. For each file chunk (or batch), call `Provider.ReviewCode()`
3. Aggregate all comments
4. Post review via existing `pull_service.SubmitReview()` with `ReviewTypeComment`
5. Post a summary comment on the PR timeline

### 3.3 Structured Output Parsing (`services/aireview/parser.go`)
The AI response should be structured (JSON). Use OpenAI `response_format` / `tools` to enforce:

```json
{
  "summary": "Overall review summary...",
  "comments": [
    {"file": "main.go", "line": 42, "severity": "warning", "body": "..."}
  ]
}
```

Fallback: if structured output fails, parse free-text response with regex heuristics.

### 3.4 Rate Limiting & Caching (`services/aireview/limiter.go`)
- Per-repo rate limiting to avoid excessive API calls
- Cache reviewed commit SHAs to avoid re-reviewing unchanged PRs
- Respect API rate limits (track tokens used)

---

## Phase 4: Testing & Integration

### 4.1 Configuration Example (`custom/conf/app.ini`)
```ini
[ai_review]
ENABLED = true
API_TOKEN = <your-openrouter-api-key>
MODEL = openai/gpt-4o
TRIGGER_ON_OPEN = true
TRIGGER_ON_UPDATE = true
```

### 4.2 Unit Tests
- `services/aireview/provider_test.go` — mock HTTP server for OpenAI API
- `services/aireview/parser_test.go` — test JSON parsing
- `services/aireview/diff_test.go` — test diff splitting
- `services/aireview/reviewer_test.go` — integration test with mock provider

### 4.3 Manual Testing
1. Build Gitea from fork
2. Run locally with `app.ini` configured
3. Open a test PR → verify AI review is posted
4. Push new commits → verify re-review

---

## File Map (New Files)

```
modules/setting/ai_review.go          — config struct & defaults
services/aireview/provider.go          — Provider interface & types
services/aireview/openai.go            — OpenAI-compatible API client
services/aireview/registry.go          — provider registry
services/aireview/notifier.go          — notification hooks (PR events)
services/aireview/queue.go             — async task queue
services/aireview/diff.go              — PR diff fetching & splitting
services/aireview/reviewer.go          — main review orchestrator
services/aireview/parser.go            — structured output parsing
services/aireview/limiter.go           — rate limiting & caching
templates/repo/ai_review_summary.tmpl  — optional summary template
web_src/css/ai-review.css              — optional styling
```

## Modified Files

```
modules/setting/setting.go             — add LoadAIRreview() call
services/notify/notify.go             — register aireview notifier
```

---

## Design Principles
- **Provider-agnostic**: Any OpenAI-compatible endpoint works (OpenRouter, OpenAI, Azure, Together, Groq, etc.)
- **Async**: AI reviews run in background queue, don't block PR creation
- **Non-intrusive**: Respects existing review permissions and privacy
- **Configurable**: All aspects tunable via `app.ini`
- **Graceful degradation**: If AI API fails, PR flow is unaffected
