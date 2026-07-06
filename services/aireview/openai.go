// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gitea.dev/modules/setting"
)

const (
	openaiChatEndpoint = "/v1/chat/completions"
)

func systemPrompt(custom string) string {
	if custom != "" {
		return custom
	}
	if p := setting.AIRreview.SystemPrompt; p != "" {
		return p
	}
	return `You are an expert code reviewer. Review the following code changes and provide constructive feedback.
Focus on:
- Bugs and logic errors
- Security vulnerabilities
- Performance issues
- Code style and best practices
- Potential edge cases
- Missing or inadequate documentation on public APIs

For each issue found, provide a suggested fix when possible.
For missing docstrings, always provide a suggested_fix with the docstring to add.
{
  "suggested_fix": {
    "old_code": "the existing code that needs to change",
    "new_code": "the fixed version of the code"
  }
}

Return your review as JSON with this structure:
{
  "summary": "Overall review summary",
  "walkthrough": [
    {"title": "Brief group title", "description": "What this group of changes does", "files": ["path/to/file.go"]}
  ],
  "architecture": "Mermaid diagram describing the architecture (or empty string)",
  "comments": [
    {"file": "path/to/file.go", "line": 42, "severity": "warning", "body": "Description of the issue", "suggested_fix": {"old_code": "...", "new_code": "..."}}
  ]
}
- Group changed files into logical walkthrough sections by concern (e.g. "Backend API changes", "Frontend UI updates").
- Generate a Mermaid architecture diagram if the changes span multiple components.
- severity must be one of: "critical", "warning", "info"
- If there are no issues, return an empty comments array.
- Omit walkthrough, architecture, and suggested_fix if not applicable.`
}

// OpenAIProvider implements the Provider interface for OpenAI-compatible APIs.
type OpenAIProvider struct {
	apiURL  string
	apiKey  string
	model   string
	timeout time.Duration
	client  *http.Client
}

// NewOpenAIProvider creates a new OpenAI-compatible provider from global settings.
func NewOpenAIProvider() *OpenAIProvider {
	return &OpenAIProvider{
		apiURL:  setting.AIRreview.APIURL,
		apiKey:  setting.AIRreview.APIToken,
		model:   setting.AIRreview.Model,
		timeout: time.Duration(setting.AIRreview.Timeout) * time.Second,
		client: &http.Client{
			Timeout: time.Duration(setting.AIRreview.Timeout) * time.Second,
		},
	}
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return "openai-compatible"
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    float64         `json:"temperature,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ReviewCode sends code diffs to the OpenAI-compatible API and returns review comments.
func (p *OpenAIProvider) ReviewCode(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("aireview: API token is not configured")
	}

	prompt := buildReviewPrompt(req)

	sysPrompt := systemPrompt(req.SystemPrompt)

	// Append path-specific instructions to system prompt
	for _, pi := range req.PathInstructions {
		sysPrompt += fmt.Sprintf("\nFor files matching %q: %s", pi.Path, pi.Instructions)
	}

	body := chatRequest{
		Model:       p.model,
		MaxTokens:   setting.AIRreview.MaxTokens,
		Temperature: setting.AIRreview.Temperature,
		ResponseFormat: &responseFormat{Type: "json_object"},
		Messages: []chatMessage{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: prompt},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("aireview: failed to marshal request: %w", err)
	}

	endpoint := p.apiURL + openaiChatEndpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("aireview: failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("aireview: API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("aireview: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aireview: API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("aireview: failed to parse response: %w", err)
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("aireview: API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("aireview: API returned no choices")
	}

	content := chatResp.Choices[0].Message.Content

	var reviewResp ReviewResponse
	if err := json.Unmarshal([]byte(content), &reviewResp); err != nil {
		return nil, fmt.Errorf("aireview: failed to parse review JSON from response: %w", err)
	}

	return &reviewResp, nil
}

// Chat sends a conversational request (no JSON mode) and returns the text response.
func (p *OpenAIProvider) Chat(ctx context.Context, messages []chatMessage) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("aireview: API token is not configured")
	}

	body := chatRequest{
		Model:       p.model,
		MaxTokens:   setting.AIRreview.MaxTokens,
		Temperature: setting.AIRreview.Temperature,
		Messages:    messages,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("aireview: failed to marshal chat request: %w", err)
	}

	endpoint := p.apiURL + openaiChatEndpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("aireview: failed to create chat request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("aireview: chat API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("aireview: failed to read chat response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("aireview: chat API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("aireview: failed to parse chat response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("aireview: chat API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("aireview: chat API returned no choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

func buildReviewPrompt(req *ReviewRequest) string {
	var b bytes.Buffer

	if len(req.Files) > 0 {
		b.WriteString("Review the following code changes for this pull request.\n")
		if req.PRTitle != "" {
			b.WriteString(fmt.Sprintf("PR Title: %s\n", req.PRTitle))
		}
		if req.PRDescription != "" {
			b.WriteString(fmt.Sprintf("PR Description: %s\n", req.PRDescription))
		}
		b.WriteString(fmt.Sprintf("\nTotal files changed: %d\n\n", len(req.Files)))

		for _, f := range req.Files {
			b.WriteString(fmt.Sprintf("### File: %s", f.Path))
			if f.Language != "" {
				b.WriteString(fmt.Sprintf(" (%s)", f.Language))
			}
			b.WriteString("\n")
			b.WriteString("```diff\n")
			patch := f.Patch
			if len(patch) > setting.AIRreview.MaxPatchSize {
				patch = patch[:setting.AIRreview.MaxPatchSize] + "\n... (patch truncated)"
			}
			b.WriteString(patch)
			if !bytes.HasSuffix([]byte(patch), []byte("\n")) {
				b.WriteString("\n")
			}
			b.WriteString("```\n\n")
		}
	} else {
		b.WriteString("Review the following code changes.\n")
		if req.PRTitle != "" {
			b.WriteString(fmt.Sprintf("PR Title: %s\n", req.PRTitle))
		}
		if req.PRDescription != "" {
			b.WriteString(fmt.Sprintf("PR Description: %s\n", req.PRDescription))
		}
		if req.FilePath != "" {
			b.WriteString(fmt.Sprintf("File: %s\n", req.FilePath))
		}
		if req.Language != "" {
			b.WriteString(fmt.Sprintf("Language: %s\n", req.Language))
		}
		b.WriteString("\n```diff\n")
		b.WriteString(req.Diff)
		if !bytes.HasSuffix([]byte(req.Diff), []byte("\n")) {
			b.WriteString("\n")
		}
		b.WriteString("```\n")
	}

	return b.String()
}
