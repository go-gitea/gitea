// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.dev/modules/setting"
)

func TestProviderInterface(t *testing.T) {
	var p Provider = NewOpenAIProvider()
	if p.Name() != "openai-compatible" {
		t.Errorf("expected name 'openai-compatible', got %q", p.Name())
	}
}

func TestRegistry(t *testing.T) {
	if !HasProvider("openrouter") {
		t.Error("expected 'openrouter' to be registered")
	}
	if !HasProvider("openai") {
		t.Error("expected 'openai' to be registered")
	}
	if HasProvider("nonexistent") {
		t.Error("expected 'nonexistent' to not be registered")
	}

	p, err := GetProvider("openrouter")
	if err != nil {
		t.Fatalf("GetProvider(openrouter) failed: %v", err)
	}
	if p.Name() != "openai-compatible" {
		t.Errorf("expected name 'openai-compatible', got %q", p.Name())
	}
}

func TestReviewCode(t *testing.T) {
	// Start a test server that mocks the OpenAI API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %q", r.Header.Get("Authorization"))
		}

		resp := map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"summary": "Looks good overall.", "walkthrough": [{"title": "Main logic", "description": "Core changes", "files": ["main.go"]}], "architecture": "graph LR; A-->B", "comments": [{"file": "main.go", "line": 10, "severity": "warning", "body": "Consider adding error handling", "suggested_fix": {"old_code": "old code", "new_code": "new code"}}]}`,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Override settings for testing
	setting.AIRreview.APIURL = server.URL
	setting.AIRreview.APIToken = "test-token"
	setting.AIRreview.Model = "test-model"
	setting.AIRreview.MaxTokens = 100
	setting.AIRreview.Temperature = 0.1
	setting.AIRreview.Timeout = 10

	provider := NewOpenAIProvider()
	resp, err := provider.ReviewCode(context.Background(), &ReviewRequest{
		Diff:     `--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,4 @@\n package main\n \n+// new code\n func main() {}`,
		FilePath: "main.go",
	})
	if err != nil {
		t.Fatalf("ReviewCode failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Summary != "Looks good overall." {
		t.Errorf("expected summary 'Looks good overall.', got %q", resp.Summary)
	}
	if len(resp.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(resp.Comments))
	}
	if resp.Comments[0].File != "main.go" {
		t.Errorf("expected file 'main.go', got %q", resp.Comments[0].File)
	}
	if resp.Comments[0].SuggestedFix == nil {
		t.Error("expected suggested_fix to be parsed")
	} else if resp.Comments[0].SuggestedFix.NewCode != "new code" {
		t.Errorf("expected new_code 'new code', got %q", resp.Comments[0].SuggestedFix.NewCode)
	}
}

func TestReviewCodeNoToken(t *testing.T) {
	setting.AIRreview.APIToken = ""
	provider := NewOpenAIProvider()
	_, err := provider.ReviewCode(context.Background(), &ReviewRequest{
		Diff: "test diff",
	})
	if err == nil {
		t.Error("expected error when no token configured")
	}
}

func TestReviewCodeAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	setting.AIRreview.APIURL = server.URL
	setting.AIRreview.APIToken = "bad-token"
	setting.AIRreview.Timeout = 10

	provider := NewOpenAIProvider()
	_, err := provider.ReviewCode(context.Background(), &ReviewRequest{
		Diff: "test diff",
	})
	if err == nil {
		t.Error("expected error for API error")
	}
}

func TestBuildReviewPrompt(t *testing.T) {
	req := &ReviewRequest{
		Diff:          "test diff",
		FilePath:      "main.go",
		PRTitle:       "Fix bug",
		PRDescription: "This fixes a critical bug",
		Language:      "Go",
	}
	prompt := buildReviewPrompt(req)
	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
}

func TestProviderRegistryDefaultProviders(t *testing.T) {
	providers := []string{"openrouter", "openai"}
	for _, name := range providers {
		p, err := GetProvider(name)
		if err != nil {
			t.Errorf("GetProvider(%q) failed: %v", name, err)
			continue
		}
		if p == nil {
			t.Errorf("GetProvider(%q) returned nil", name)
		}
	}
}

func TestMergeRepoConfigNil(t *testing.T) {
	sysPrompt, exclPaths, pathInst, checks := MergeRepoConfig("global prompt", []string{"vendor/*"}, nil)
	if sysPrompt != "global prompt" {
		t.Errorf("expected global prompt, got %q", sysPrompt)
	}
	if len(exclPaths) != 1 || exclPaths[0] != "vendor/*" {
		t.Errorf("expected [vendor/*], got %v", exclPaths)
	}
	if len(pathInst) != 0 {
		t.Errorf("expected 0 path instructions, got %d", len(pathInst))
	}
	if len(checks) != 0 {
		t.Errorf("expected 0 custom checks, got %d", len(checks))
	}
}

func TestMergeRepoConfigOverride(t *testing.T) {
	repoPrompt := "repo prompt"
	repoCfg := &RepoConfig{
		SystemPrompt: &repoPrompt,
		ExcludePaths: []string{"node_modules/**"},
		PathInstructions: []PathInstruction{
			{Path: "src/*.go", Instructions: "Be strict"},
		},
		CustomChecks: []string{"All functions must have tests"},
	}
	sysPrompt, exclPaths, pathInst, checks := MergeRepoConfig("global prompt", []string{"vendor/*"}, repoCfg)
	if sysPrompt != "repo prompt" {
		t.Errorf("expected 'repo prompt', got %q", sysPrompt)
	}
	if len(exclPaths) != 1 || exclPaths[0] != "node_modules/**" {
		t.Errorf("expected [node_modules/**], got %v", exclPaths)
	}
	if len(pathInst) != 1 || pathInst[0].Path != "src/*.go" {
		t.Errorf("expected 1 path instruction, got %d", len(pathInst))
	}
	if len(checks) != 1 || checks[0] != "All functions must have tests" {
		t.Errorf("expected 1 custom check, got %d", len(checks))
	}
}

func TestMergeRepoConfigPartialOverride(t *testing.T) {
	repoCfg := &RepoConfig{
		ExcludePaths: []string{"generated/**"},
	}
	sysPrompt, exclPaths, pathInst, checks := MergeRepoConfig("global prompt", nil, repoCfg)
	if sysPrompt != "global prompt" {
		t.Errorf("expected 'global prompt', got %q", sysPrompt)
	}
	if len(exclPaths) != 1 || exclPaths[0] != "generated/**" {
		t.Errorf("expected [generated/**], got %v", exclPaths)
	}
	if len(pathInst) != 0 {
		t.Errorf("expected 0 path instructions, got %d", len(pathInst))
	}
	if len(checks) != 0 {
		t.Errorf("expected 0 custom checks, got %d", len(checks))
	}
}

func TestExtractImports(t *testing.T) {
	imports := extractImports("web/handler.go", "package web\n\nimport (\n\t\"gitea.dev/services/core\"\n)\n")
	if !imports["gitea.dev/services/core"] {
		t.Error("expected gitea.dev/services/core to be found")
	}
}

func TestImportMatchesPath(t *testing.T) {
	if !importMatchesPath("gitea.dev/services/core", "services/core/engine.go") {
		t.Error("expected gitea.dev/services/core to match services/core/engine.go")
	}
	if !importMatchesPath("gitea.dev/web", "web/handler.go") {
		t.Error("expected gitea.dev/web to match web/handler.go")
	}
	if importMatchesPath("fmt", "main.go") {
		t.Error("expected fmt not to match main.go")
	}
}

func TestSortFilesByDependency(t *testing.T) {
	files := []FileDiff{
		{Path: "services/core/engine.go", Patch: "package core\n"},
		{Path: "web/handler.go", Patch: "package web\n\nimport (\n\t\"gitea.dev/services/core\"\n)\n"},
		{Path: "main.go", Patch: "package main\n\nimport (\n\t\"gitea.dev/web\"\n)\n"},
	}
	ordered := SortFilesByDependency(files)
	if len(ordered) != 3 {
		t.Fatalf("expected 3 files, got %d", len(ordered))
	}
	// engine.go should come before handler.go (which imports it)
	if ordered[0].Path != "services/core/engine.go" {
		t.Errorf("expected engine.go first, got %s", ordered[0].Path)
	}
	// main.go should be last (depends on web)
	if ordered[2].Path != "main.go" {
		t.Errorf("expected main.go last, got %s", ordered[2].Path)
	}
}

func TestLearnings(t *testing.T) {
	repoID := int64(42)

	// Initially no learnings
	if len(GetLearnings(repoID)) != 0 {
		t.Error("expected 0 learnings initially")
	}

	// Add a learning
	AddLearning(repoID, Learning{PathGlob: "*.go", Instruction: "Use errors.Is instead of == for sentinel errors"})
	if len(GetLearnings(repoID)) != 1 {
		t.Fatal("expected 1 learning")
	}

	// Build prompt
	prompt := BuildLearningsPrompt(repoID)
	if !strings.Contains(prompt, "errors.Is") {
		t.Error("expected prompt to contain learning")
	}

	// Detect from chat message
	DetectAndStoreLearnings(repoID, "ignore type assertions, they are intentional")
	if len(GetLearnings(repoID)) != 2 {
		t.Errorf("expected 2 learnings, got %d", len(GetLearnings(repoID)))
	}

	// Detect explicit learn: prefix
	DetectAndStoreLearnings(repoID, "learn: *.py: Use f-strings instead of % formatting")
	if len(GetLearnings(repoID)) != 3 {
		t.Errorf("expected 3 learnings, got %d", len(GetLearnings(repoID)))
	}
}

func TestSortFilesByDependencyNoImports(t *testing.T) {
	files := []FileDiff{
		{Path: "b/b.go", Patch: `package b`},
		{Path: "a/a.go", Patch: `package a`},
	}
	ordered := SortFilesByDependency(files)
	if len(ordered) != 2 {
		t.Fatalf("expected 2 files, got %d", len(ordered))
	}
	// Should be sorted by depth then alphabetically
	if ordered[0].Path != "a/a.go" {
		t.Errorf("expected a/a.go first, got %s", ordered[0].Path)
	}
}

func TestFormatCommentBodyWithFix(t *testing.T) {
	body := formatCommentBody(aiComment{
		ReviewComment: ReviewComment{
			File:     "main.go",
			Line:     10,
			Severity: "critical",
			Body:     "Potential nil pointer dereference",
			SuggestedFix: &SuggestedFix{
				OldCode: "if err != nil",
				NewCode: "if err != nil { return }",
			},
		},
	})
	if !strings.Contains(body, "Suggested fix") {
		t.Error("expected suggested fix section")
	}
	if !strings.Contains(body, "CRITICAL") {
		t.Error("expected CRITICAL severity tag")
	}
	if !strings.Contains(body, "if err != nil { return }") {
		t.Error("expected suggested new code")
	}
}

func TestFormatReviewBodyWalkthrough(t *testing.T) {
	comments := []aiComment{
		{ReviewComment: ReviewComment{File: "main.go", Line: 10, Severity: "warning", Body: "Check this"}},
	}
	body := formatReviewBody(&ReviewResponse{
		Summary: "Good PR",
		Walkthrough: []WalkthroughSection{
			{Title: "Core", Description: "Core changes", Files: []string{"main.go"}},
		},
		Architecture: "graph LR; A-->B",
		Comments: []ReviewComment{
			{File: "main.go", Line: 10, Severity: "warning", Body: "Check this"},
		},
	}, comments)
	if body == "" {
		t.Error("expected non-empty body")
	}
	if !strings.Contains(body, "Change Walkthrough") {
		t.Error("expected walkthrough section")
	}
	if !strings.Contains(body, "Core") {
		t.Error("expected walkthrough title")
	}
	if !strings.Contains(body, "mermaid") {
		t.Error("expected mermaid diagram")
	}
}
