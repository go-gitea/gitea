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
	openaiChatEndpoint  = "/v1/chat/completions"
	defaultSystemPrompt = `You are an expert code reviewer. Review the following code diff and provide constructive feedback.
Focus on:
- Bugs and logic errors
- Security vulnerabilities
- Performance issues
- Code style and best practices
- Potential edge cases

Return your review as JSON with this structure:
{
  "summary": "Overall review summary",
  "comments": [
    {"file": "path/to/file.go", "line": 42, "severity": "warning", "body": "Description of the issue"}
  ]
}
severity must be one of: "critical", "warning", "info"
If there are no issues, return an empty comments array.`
)

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

// ReviewCode sends a code diff to the OpenAI-compatible API and returns review comments.
func (p *OpenAIProvider) ReviewCode(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("aireview: API token is not configured")
	}

	prompt := buildReviewPrompt(req)

	body := chatRequest{
		Model:          p.model,
		MaxTokens:      setting.AIRreview.MaxTokens,
		Temperature:    setting.AIRreview.Temperature,
		ResponseFormat: &responseFormat{Type: "json_object"},
		Messages: []chatMessage{
			{Role: "system", Content: defaultSystemPrompt},
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

func buildReviewPrompt(req *ReviewRequest) string {
	prompt := fmt.Sprintf("Review the following code changes.\n")
	if req.PRTitle != "" {
		prompt += fmt.Sprintf("PR Title: %s\n", req.PRTitle)
	}
	if req.PRDescription != "" {
		prompt += fmt.Sprintf("PR Description: %s\n", req.PRDescription)
	}
	if req.FilePath != "" {
		prompt += fmt.Sprintf("File: %s\n", req.FilePath)
	}
	if req.Language != "" {
		prompt += fmt.Sprintf("Language: %s\n", req.Language)
	}
	prompt += fmt.Sprintf("\n```diff\n%s\n```\n", req.Diff)
	return prompt
}
