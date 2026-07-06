// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	issue_service "gitea.dev/services/issue"
)

const chatBotMention = "@aireview"

// conversation stores chat history per PR.
type conversation struct {
	mu  sync.Mutex
	msgs map[int64][]chatMessage // PRID → message history
}

var conversations = &conversation{msgs: make(map[int64][]chatMessage)}

func systemChatPrompt() string {
	return "You are an AI code review assistant integrated with Gitea. " +
		"You help developers understand code changes, answer questions about pull requests, " +
		"and provide code review feedback. Be concise, accurate, and helpful. " +
		"When referencing code, include file paths and line numbers."
}

// HandlePRComment processes a comment on a PR issue and responds if it mentions the bot.
func HandlePRComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, comment *issues_model.Comment) error {
	if !issue.IsPull || !setting.AIRreview.Enabled {
		return nil
	}

	// Only respond to regular comments that mention the bot
	if comment.Type != issues_model.CommentTypeComment {
		return nil
	}
	if !strings.Contains(comment.Content, chatBotMention) {
		return nil
	}

	pr, err := issues_model.GetPullRequestByIssueID(ctx, issue.ID)
	if err != nil {
		return fmt.Errorf("load PR: %w", err)
	}
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return fmt.Errorf("load base repo: %w", err)
	}

	files, err := GetPRDiff(ctx, pr)
	if err != nil {
		return fmt.Errorf("get PR diff: %w", err)
	}

	// Build diff summary for context
	var diffBuf bytes.Buffer
	diffBuf.WriteString(fmt.Sprintf("PR #%d: %s\n", pr.Index, issue.Title))
	if issue.Content != "" {
		diffBuf.WriteString(fmt.Sprintf("Description: %s\n", issue.Content))
	}
	diffBuf.WriteString(fmt.Sprintf("Files changed (%d):\n", len(files)))
	for _, f := range files {
		patch := f.Patch
		if len(patch) > 2000 {
			patch = patch[:2000] + "\n... (truncated)"
		}
		diffBuf.WriteString(fmt.Sprintf("\n### %s\n```diff\n%s\n```\n", f.Path, patch))
	}

	diffContext := diffBuf.String()

	// Get or create conversation history
	conversations.mu.Lock()
	history := conversations.msgs[issue.ID]
	if history == nil {
		history = []chatMessage{
			{Role: "system", Content: systemChatPrompt() + "\n\nHere is the PR diff context:\n" + diffContext},
		}
	}
	// Append user message
	userMsg := strings.TrimSpace(strings.ReplaceAll(comment.Content, chatBotMention, ""))
	// Detect and store learnings from feedback
	DetectAndStoreLearnings(pr.BaseRepo.ID, userMsg)
	if userMsg == "" {
		userMsg = "Explain these changes."
	}
	history = append(history, chatMessage{Role: "user", Content: userMsg})
	conversations.msgs[issue.ID] = history
	conversations.mu.Unlock()

	// Call AI
	provider, err := GetProvider(setting.AIRreview.Provider)
	if err != nil {
		return fmt.Errorf("get provider: %w", err)
	}

	// Build chat request using the provider's API
	openAIProvider, ok := provider.(*OpenAIProvider)
	if !ok {
		return fmt.Errorf("chat not supported for provider %s", provider.Name())
	}

	aiResp, err := openAIProvider.Chat(ctx, history)
	if err != nil {
		return fmt.Errorf("AI chat failed: %w", err)
	}

	// Store AI response in conversation history
	conversations.mu.Lock()
	conversations.msgs[issue.ID] = append(conversations.msgs[issue.ID], chatMessage{Role: "assistant", Content: aiResp})
	conversations.mu.Unlock()

	// Post reply as comment
	branchLink := fmt.Sprintf("https://git.example.com/%s/pulls/%d", pr.BaseRepo.FullName(), pr.Index)
	response := fmt.Sprintf("> **%s** asked:\n> %s\n\n---\n\n%s\n\n---\n*AI code review assistant — [PR #%d](%s)*",
		doer.DisplayName(), userMsg, aiResp, pr.Index, branchLink)

	// Trim response if too long
	if len(response) > 50000 {
		response = response[:50000] + "\n\n... (response truncated)"
	}

	_, err = issue_service.CreateIssueComment(ctx, doer, pr.BaseRepo, issue, response, nil)
	if err != nil {
		return fmt.Errorf("create comment: %w", err)
	}

	log.Info("aireview: responded to chat in PR #%d (comment %d)", pr.Index, comment.ID)
	return nil
}
