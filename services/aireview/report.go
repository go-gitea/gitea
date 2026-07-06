// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"context"
	"fmt"
	"strings"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/setting"
)

// GenerateSprintReport creates a summary of recent open pull requests.
func GenerateSprintReport(ctx context.Context, repo *repo_model.Repository) (string, error) {
	prs, _, err := issues_model.PullRequests(ctx, repo.ID, &issues_model.PullRequestsOptions{
		State:    "open",
		SortType: "recentupdate",
	})
	if err != nil {
		return "", fmt.Errorf("list PRs: %w", err)
	}

	if len(prs) == 0 {
		return "No open pull requests.", nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Current Pull Requests\n\n"))
	b.WriteString(fmt.Sprintf("Total open PRs: %d\n\n", len(prs)))

	for _, pr := range prs {
		b.WriteString(fmt.Sprintf("- **#%d**: %s\n", pr.Index, pr.Issue.Title))
	}

	provider, err := GetProvider(setting.AIRreview.Provider)
	if err != nil {
		return b.String(), nil
	}

	summary, err := provider.Chat(ctx, []ChatMessage{
		{Role: "system", Content: "You are a sprint report generator. Summarize the following pull requests in a clear, organized format."},
		{Role: "user", Content: "Summarize these pull requests:\n\n" + b.String()},
	})
	if err != nil {
		return b.String(), nil
	}

	return summary, nil
}
