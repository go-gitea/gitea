// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
)

const (
	anthropicAPIEndpoint = "/v1/messages"
	anthropicVersion     = "2023-06-01"
)

// AnthropicProvider implements the Provider interface for Anthropic's Claude API.
type AnthropicProvider struct {
	apiURL  string
	apiKey  string
	model   string
	timeout time.Duration
	client  *http.Client
}

// NewAnthropicProvider creates a new Anthropic provider from global settings.
func NewAnthropicProvider() *AnthropicProvider {
	return &AnthropicProvider{
		apiURL:  setting.AIRreview.APIURL,
		apiKey:  setting.AIRreview.APIToken,
		model:   setting.AIRreview.Model,
		timeout: time.Duration(setting.AIRreview.Timeout) * time.Second,
		client: &http.Client{
			Timeout: time.Duration(setting.AIRreview.Timeout) * time.Second,
		},
	}
}

func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
	Error   *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *AnthropicProvider) ReviewCode(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error) {
	if p.apiKey == "" {
		return nil, errors.New("aireview: API token is not configured")
	}

	prompt := buildReviewPrompt(req)
	sysPrompt := systemPrompt(req.SystemPrompt)

	var sysPromptBuilder strings.Builder
	sysPromptBuilder.WriteString(sysPrompt)
	for _, pi := range req.PathInstructions {
		fmt.Fprintf(&sysPromptBuilder, "\nFor files matching %q: %s", pi.Path, pi.Instructions)
	}
	sysPrompt = sysPromptBuilder.String()

	if refs := extractIssueRefs(req.PRDescription); refs != "" {
		prompt += "\nReferenced issues: " + refs + "\n"
	}
	if len(req.Files) > 0 && req.LinterConfigs != "" {
		prompt += "\n" + req.LinterConfigs
	}
	if len(req.CustomChecks) > 0 {
		var promptBuilder strings.Builder
		promptBuilder.WriteString(prompt)
		promptBuilder.WriteString("\n\n**Pre-merge checks to evaluate:**\n")
		for i, check := range req.CustomChecks {
			fmt.Fprintf(&promptBuilder, "%d. %s\n", i+1, check)
		}
		promptBuilder.WriteString("\nFor each check, return a check_results entry with check name, passed (bool), and details.")
		prompt = promptBuilder.String()
	}

	body := anthropicRequest{
		Model:       p.model,
		MaxTokens:   setting.AIRreview.MaxTokens,
		System:      sysPrompt,
		Temperature: setting.AIRreview.Temperature,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("aireview: failed to marshal request: %w", err)
	}

	endpoint := strings.TrimRight(p.apiURL, "/") + anthropicAPIEndpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("aireview: failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

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

	var aResp anthropicResponse
	if err := json.Unmarshal(respBody, &aResp); err != nil {
		return nil, fmt.Errorf("aireview: failed to parse response: %w", err)
	}

	if aResp.Error != nil {
		return nil, fmt.Errorf("aireview: API error: %s", aResp.Error.Message)
	}

	var textBuilder strings.Builder
	for _, block := range aResp.Content {
		if block.Type == "text" {
			textBuilder.WriteString(block.Text)
		}
	}
	text := textBuilder.String()

	if text == "" {
		return nil, errors.New("aireview: API returned empty content")
	}

	var reviewResp ReviewResponse
	if err := json.Unmarshal([]byte(text), &reviewResp); err != nil {
		return nil, fmt.Errorf("aireview: failed to parse review JSON from response: %w", err)
	}

	return &reviewResp, nil
}

func (p *AnthropicProvider) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	if p.apiKey == "" {
		return "", errors.New("aireview: API token is not configured")
	}

	var systemMsgBuilder strings.Builder
	var anthropicMsgs []anthropicMessage
	for _, m := range messages {
		if m.Role == "system" {
			systemMsgBuilder.WriteString(m.Content)
			systemMsgBuilder.WriteString("\n")
		} else {
			anthropicMsgs = append(anthropicMsgs, anthropicMessage(m))
		}
	}
	systemMsg := systemMsgBuilder.String()

	body := anthropicRequest{
		Model:       p.model,
		MaxTokens:   setting.AIRreview.MaxTokens,
		System:      strings.TrimSpace(systemMsg),
		Temperature: setting.AIRreview.Temperature,
		Messages:    anthropicMsgs,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("aireview: failed to marshal chat request: %w", err)
	}

	endpoint := strings.TrimRight(p.apiURL, "/") + anthropicAPIEndpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("aireview: failed to create chat request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

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

	var aResp anthropicResponse
	if err := json.Unmarshal(respBody, &aResp); err != nil {
		return "", fmt.Errorf("aireview: failed to parse chat response: %w", err)
	}

	if aResp.Error != nil {
		return "", fmt.Errorf("aireview: chat API error: %s", aResp.Error.Message)
	}

	var textBuilder strings.Builder
	for _, block := range aResp.Content {
		if block.Type == "text" {
			textBuilder.WriteString(block.Text)
		}
	}

	return textBuilder.String(), nil
}
