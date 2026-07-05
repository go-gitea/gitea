// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"context"
	"fmt"
	"sort"
	"strings"

	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	pull_service "gitea.dev/services/pull"
)

// RunReview executes an AI code review on a pull request.
func RunReview(ctx context.Context, task *AIRreviewTask) error {
	if !setting.AIRreview.Enabled {
		return nil
	}

	pr, err := issues_model.GetPullRequestByID(ctx, task.PRID)
	if err != nil {
		return fmt.Errorf("load PR %d: %w", task.PRID, err)
	}

	if err := pr.LoadIssue(ctx); err != nil {
		return fmt.Errorf("load issue: %w", err)
	}
	if err := pr.Issue.LoadRepo(ctx); err != nil {
		return fmt.Errorf("load repo: %w", err)
	}
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return fmt.Errorf("load base repo: %w", err)
	}

	doer, err := user_model.GetUserByID(ctx, pr.BaseRepo.OwnerID)
	if err != nil {
		return fmt.Errorf("get repo owner: %w", err)
	}

	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, pr.BaseRepo)
	if err != nil {
		return fmt.Errorf("open git repo: %w", err)
	}
	defer closer.Close()

	headCommitID, err := gitRepo.GetRefCommitID(pr.GetGitHeadRefName())
	if err != nil {
		return fmt.Errorf("get head commit: %w", err)
	}

	files, err := GetPRDiff(ctx, pr)
	if err != nil {
		return fmt.Errorf("get PR diff: %w", err)
	}

	if len(files) == 0 {
		log.Info("aireview: PR %d has no files to review", task.PRID)
		return nil
	}

	provider, err := GetProvider(setting.AIRreview.Provider)
	if err != nil {
		return fmt.Errorf("get provider: %w", err)
	}

	var allComments []ReviewComment
	var summaries []string

	for _, file := range files {
		if file.Patch == "" {
			continue
		}
		patch := file.Patch
		if len(patch) > setting.AIRreview.MaxPatchSize {
			patch = patch[:setting.AIRreview.MaxPatchSize] + "\n... (patch truncated)"
		}

		resp, err := provider.ReviewCode(ctx, &ReviewRequest{
			Diff:     patch,
			FilePath: file.Path,
			Language: file.Language,
		})
		if err != nil {
			log.Error("aireview: failed to review %s: %v", file.Path, err)
			continue
		}

		for i := range resp.Comments {
			resp.Comments[i].File = file.Path
		}
		allComments = append(allComments, resp.Comments...)
		if resp.Summary != "" {
			summaries = append(summaries, resp.Summary)
		}
	}

	if len(allComments) == 0 && len(summaries) == 0 {
		log.Info("aireview: no issues found in PR %d", task.PRID)
		return nil
	}

	reviewContent := formatReviewOutput(summaries, allComments)

	_, _, err = pull_service.SubmitReview(ctx, doer, gitRepo, pr.Issue,
		issues_model.ReviewTypeComment,
		reviewContent,
		headCommitID,
		nil,
	)
	if err != nil {
		return fmt.Errorf("submit review: %w", err)
	}

	log.Info("aireview: completed review of PR %d with %d comments", task.PRID, len(allComments))
	return nil
}

func formatReviewOutput(summaries []string, comments []ReviewComment) string {
	var b strings.Builder

	b.WriteString("### AI Code Review\n\n")

	if len(summaries) > 0 {
		b.WriteString("**Summary:**\n")
		for _, s := range summaries {
			b.WriteString(s)
			b.WriteString("\n\n")
		}
	}

	sort.Slice(comments, func(i, j int) bool {
		if comments[i].File != comments[j].File {
			return comments[i].File < comments[j].File
		}
		return comments[i].Line < comments[j].Line
	})

	severityOrder := map[string]int{"critical": 0, "warning": 1, "info": 2}
	sort.SliceStable(comments, func(i, j int) bool {
		return severityOrder[comments[i].Severity] < severityOrder[comments[j].Severity]
	})

	grouped := make(map[string][]ReviewComment)
	for _, c := range comments {
		key := c.File
		grouped[key] = append(grouped[key], c)
	}

	b.WriteString("**Findings:**\n\n")
	for _, file := range sortedKeys(grouped) {
		fileComments := grouped[file]
		for _, c := range fileComments {
			severityTag := ""
			switch c.Severity {
			case "critical":
				severityTag = "[CRITICAL]"
			case "warning":
				severityTag = "[WARNING]"
			default:
				severityTag = "[INFO]"
			}
			b.WriteString(fmt.Sprintf("- %s %s:%d %s\n", severityTag, file, c.Line, c.Body))
		}
	}

	return b.String()
}

func sortedKeys(m map[string][]ReviewComment) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
