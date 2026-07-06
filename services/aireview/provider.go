// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import "context"

// SuggestedFix represents a code change suggested by the AI.
type SuggestedFix struct {
	OldCode string `json:"old_code"`
	NewCode string `json:"new_code"`
}

// ReviewComment represents a single code review comment from the AI.
type ReviewComment struct {
	File         string        `json:"file"`
	Line         int           `json:"line"`
	Body         string        `json:"body"`
	Severity     string        `json:"severity"` // "critical", "warning", "info"
	SuggestedFix *SuggestedFix `json:"suggested_fix,omitempty"`
}

// ReviewRequest contains the context sent to the AI provider.
type ReviewRequest struct {
	Files           []FileDiff        // all changed files (preferred over single Diff)
	Diff            string            // single-file fallback
	FilePath        string            // single-file fallback
	CommitSHA        string
	PRTitle          string
	PRDescription    string
	Language         string            // single-file fallback
	SystemPrompt     string            // per-repo system prompt override
	PathInstructions []PathInstruction  // per-path review instructions
	CustomChecks     []string           // pre-merge checks to evaluate
}

// WalkthroughSection describes a logical group of changes in the PR.
type WalkthroughSection struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
}

// CheckResult represents the result of a single custom pre-merge check.
type CheckResult struct {
	Check   string `json:"check"`
	Passed  bool   `json:"passed"`
	Details string `json:"details,omitempty"`
}

// ReviewResponse is the structured result from the AI provider.
type ReviewResponse struct {
	Summary      string              `json:"summary"`
	Walkthrough  []WalkthroughSection `json:"walkthrough"`
	Architecture string              `json:"architecture"` // Mermaid diagram
	Comments     []ReviewComment      `json:"comments"`
	CheckResults []CheckResult        `json:"check_results"`
}

// Provider defines the interface for AI code review providers.
type Provider interface {
	// ReviewCode sends code diffs to the AI and returns review comments.
	ReviewCode(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error)
	// Name returns the provider name for identification.
	Name() string
}
