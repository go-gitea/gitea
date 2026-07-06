// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"context"
	"fmt"
	"strings"

	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	pull_service "gitea.dev/services/pull"
)

type aiComment struct {
	ReviewComment
	Inlined bool
}

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

	var allComments []aiComment
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

		for _, c := range resp.Comments {
			c.File = file.Path
			allComments = append(allComments, aiComment{ReviewComment: c})
		}
		if resp.Summary != "" {
			summaries = append(summaries, resp.Summary)
		}
	}

	if len(allComments) == 0 && len(summaries) == 0 {
		log.Info("aireview: no issues found in PR %d", task.PRID)
		return nil
	}

	inlineCount := 0
	for i, c := range allComments {
		if c.Line <= 0 {
			continue
		}
		_, err := pull_service.CreateCodeComment(ctx, doer, gitRepo, pr.Issue,
			int64(c.Line),
			formatCommentBody(c),
			c.File,
			true, // pendingReview — add to pending review
			0,    // replyReviewID
			headCommitID,
			nil, // attachments
		)
		if err != nil {
			log.Warn("aireview: failed to create inline comment at %s:%d: %v", c.File, c.Line, err)
			continue
		}
		allComments[i].Inlined = true
		inlineCount++
	}

	reviewContent := formatReviewBody(summaries, allComments)

	_, _, err = pull_service.SubmitReview(ctx, doer, gitRepo, pr.Issue,
		issues_model.ReviewTypeComment,
		reviewContent,
		headCommitID,
		nil,
	)
	if err != nil {
		return fmt.Errorf("submit review: %w", err)
	}

	log.Info("aireview: completed review of PR %d — %d inline, %d in summary", task.PRID, inlineCount, len(allComments)-inlineCount)
	return nil
}

func formatCommentBody(c aiComment) string {
	switch c.Severity {
	case "critical":
		return fmt.Sprintf("**[CRITICAL]** %s", c.Body)
	case "warning":
		return fmt.Sprintf("**[WARNING]** %s", c.Body)
	default:
		return c.Body
	}
}

func formatReviewBody(summaries []string, comments []aiComment) string {
	var b strings.Builder

	b.WriteString("### AI Code Review\n\n")

	if len(summaries) > 0 {
		b.WriteString("**Overview:**\n")
		for _, s := range summaries {
			b.WriteString(s)
			b.WriteString("\n\n")
		}
	}

	var nonInlined []aiComment
	for _, c := range comments {
		if !c.Inlined {
			nonInlined = append(nonInlined, c)
		}
	}

	if len(nonInlined) > 0 {
		b.WriteString("**Additional findings (no inline location available):**\n")
		for _, c := range nonInlined {
			severityTag := ""
			switch c.Severity {
			case "critical":
				severityTag = "[CRITICAL]"
			case "warning":
				severityTag = "[WARNING]"
			default:
				severityTag = "[INFO]"
			}
			loc := ""
			if c.File != "" {
				loc = fmt.Sprintf(" %s:%d", c.File, c.Line)
			}
			b.WriteString(fmt.Sprintf("- %s%s %s\n", severityTag, loc, c.Body))
		}
	}

	return strings.TrimRight(b.String(), "\n ")
}
