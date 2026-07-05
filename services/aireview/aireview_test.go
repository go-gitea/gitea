// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
						"content": `{"summary": "Looks good overall.", "comments": [{"file": "main.go", "line": 10, "severity": "warning", "body": "Consider adding error handling"}]}`,
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
