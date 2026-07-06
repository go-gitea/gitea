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
	geminiGenerateEndpoint = "/v1beta/models/%s:generateContent"
)

// GeminiProvider implements the Provider interface for Google's Gemini API.
type GeminiProvider struct {
	apiURL  string
	apiKey  string
	model   string
	timeout time.Duration
	client  *http.Client
}

// NewGeminiProvider creates a new Gemini provider from global settings.
func NewGeminiProvider() *GeminiProvider {
	return &GeminiProvider{
		apiURL:  setting.AIRreview.APIURL,
		apiKey:  setting.AIRreview.APIToken,
		model:   setting.AIRreview.Model,
		timeout: time.Duration(setting.AIRreview.Timeout) * time.Second,
		client: &http.Client{
			Timeout: time.Duration(setting.AIRreview.Timeout) * time.Second,
		},
	}
}

func (p *GeminiProvider) Name() string {
	return "gemini"
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiRequest struct {
	Contents          []geminiContent `json:"contents"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	GenerationConfig  geminiGenConfig `json:"generationConfig"`
}

type geminiGenConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
	Error      *geminiAPIError   `json:"error,omitempty"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

type geminiAPIError struct {
	Message string `json:"message"`
}

func (p *GeminiProvider) ReviewCode(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error) {
	if p.apiKey == "" {
		return nil, errors.New("aireview: API token is not configured")
	}

	prompt := buildReviewPrompt(req)
	sysPrompt := systemPrompt(req.SystemPrompt)

	var sysPromptBuilder strings.Builder
	sysPromptBuilder.WriteString(sysPrompt)
	for _, pi := range req.PathInstructions {
		sysPromptBuilder.WriteString(fmt.Sprintf("\nFor files matching %q: %s", pi.Path, pi.Instructions))
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
			promptBuilder.WriteString(fmt.Sprintf("%d. %s\n", i+1, check))
		}
		promptBuilder.WriteString("\nFor each check, return a check_results entry with check name, passed (bool), and details.")
		prompt = promptBuilder.String()
	}

	body := geminiRequest{
		Contents: []geminiContent{
			{Role: "user", Parts: []geminiPart{{Text: prompt}}},
		},
		GenerationConfig: geminiGenConfig{
			MaxOutputTokens: setting.AIRreview.MaxTokens,
			Temperature:     setting.AIRreview.Temperature,
		},
	}

	if sysPrompt != "" {
		body.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: sysPrompt}},
		}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("aireview: failed to marshal request: %w", err)
	}

	model := p.model
	if model == "" {
		model = "gemini-2.0-flash"
	}
	baseURL := strings.TrimRight(p.apiURL, "/")
	endpoint := fmt.Sprintf(baseURL+geminiGenerateEndpoint, model) + "?key=" + p.apiKey

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("aireview: failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

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

	var gResp geminiResponse
	if err := json.Unmarshal(respBody, &gResp); err != nil {
		return nil, fmt.Errorf("aireview: failed to parse response: %w", err)
	}

	if gResp.Error != nil {
		return nil, fmt.Errorf("aireview: API error: %s", gResp.Error.Message)
	}

	if len(gResp.Candidates) == 0 {
		return nil, errors.New("aireview: API returned no candidates")
	}

	var textBuilder strings.Builder
	for _, part := range gResp.Candidates[0].Content.Parts {
		textBuilder.WriteString(part.Text)
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

func (p *GeminiProvider) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	if p.apiKey == "" {
		return "", errors.New("aireview: API token is not configured")
	}

	var systemMsgBuilder strings.Builder
	var contents []geminiContent
	for _, m := range messages {
		if m.Role == "system" {
			systemMsgBuilder.WriteString(m.Content)
			systemMsgBuilder.WriteString("\n")
		} else {
			role := m.Role
			if role == "assistant" {
				role = "model"
			}
			contents = append(contents, geminiContent{
				Role:  role,
				Parts: []geminiPart{{Text: m.Content}},
			})
		}
	}
	systemMsg := systemMsgBuilder.String()

	body := geminiRequest{
		Contents: contents,
		GenerationConfig: geminiGenConfig{
			MaxOutputTokens: setting.AIRreview.MaxTokens,
			Temperature:     setting.AIRreview.Temperature,
		},
	}

	if systemMsg != "" {
		body.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: strings.TrimSpace(systemMsg)}},
		}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("aireview: failed to marshal chat request: %w", err)
	}

	model := p.model
	if model == "" {
		model = "gemini-2.0-flash"
	}
	baseURL := strings.TrimRight(p.apiURL, "/")
	endpoint := fmt.Sprintf(baseURL+geminiGenerateEndpoint, model) + "?key=" + p.apiKey

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("aireview: failed to create chat request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

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

	var gResp geminiResponse
	if err := json.Unmarshal(respBody, &gResp); err != nil {
		return "", fmt.Errorf("aireview: failed to parse chat response: %w", err)
	}

	if gResp.Error != nil {
		return "", fmt.Errorf("aireview: chat API error: %s", gResp.Error.Message)
	}

	if len(gResp.Candidates) == 0 {
		return "", errors.New("aireview: chat API returned no candidates")
	}

	var textBuilder strings.Builder
	for _, part := range gResp.Candidates[0].Content.Parts {
		textBuilder.WriteString(part.Text)
	}

	return textBuilder.String(), nil
}
