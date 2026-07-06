// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import "context"

// ReviewComment represents a single code review comment from the AI.
type ReviewComment struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Body     string `json:"body"`
	Severity string `json:"severity"` // "critical", "warning", "info"
}

// ReviewRequest contains the context sent to the AI provider.
type ReviewRequest struct {
	Files         []FileDiff // all changed files (preferred over single Diff)
	Diff          string     // single-file fallback
	FilePath      string     // single-file fallback
	CommitSHA     string
	PRTitle       string
	PRDescription string
	Language      string // single-file fallback
}

// ReviewResponse is the structured result from the AI provider.
type ReviewResponse struct {
	Summary  string          `json:"summary"`
	Comments []ReviewComment `json:"comments"`
}

// Provider defines the interface for AI code review providers.
type Provider interface {
	// ReviewCode sends code diffs to the AI and returns review comments.
	ReviewCode(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error)
	// Name returns the provider name for identification.
	Name() string
}
