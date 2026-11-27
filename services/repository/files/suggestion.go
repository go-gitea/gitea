// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"bytes"
	"context"
	"errors"
	"strings"

	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	pull_service "gitea.dev/services/pull"
)

type ApplyCodeSuggestionsOptions struct {
	Suggestions  []pull_service.CodeSuggestionVersion
	HeadCommitID string
	Summary      string
	Description  string
}

func ApplyCodeSuggestions(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, opts ApplyCodeSuggestionsOptions) error {
	changes, err := pull_service.PrepareCodeSuggestionChanges(ctx, doer, pr, opts.Suggestions, opts.HeadCommitID)
	if err != nil {
		return err
	}

	files := make([]*ChangeRepoFile, 0, len(changes.Files))
	for _, file := range changes.Files {
		files = append(files, &ChangeRepoFile{
			Operation: "update", TreePath: file.Path, ContentReader: bytes.NewReader(file.Content), SHA: file.BlobID,
		})
	}

	result, err := ChangeRepoFiles(ctx, pr.HeadRepo, doer, &ChangeRepoFilesOptions{
		OldBranch: pr.HeadBranch, LastCommitID: opts.HeadCommitID,
		Files: files, Message: codeSuggestionsCommitMessage(doer, opts.Summary, opts.Description, changes.Comments),
	})
	if err != nil {
		if git.IsErrPushOutOfDate(err) || pull_service.IsErrSHADoesNotMatch(err) || errors.Is(err, util.ErrNotExist) {
			return pull_service.ErrCodeSuggestionHeadChanged
		}
		if rejected, ok := errors.AsType[*git.ErrPushRejected](err); ok {
			if rejected.Message == "" {
				return util.ErrorWrapTranslatable(util.ErrPermissionDenied, "repo.editor.push_rejected_no_message")
			}
			return util.ErrorWrapTranslatable(util.ErrPermissionDenied, "repo.diff.suggestion.push_rejected", rejected.Message)
		}
		return err
	}

	ctx = context.WithoutCancel(ctx)
	if canResolve, err := issues_model.CanMarkConversation(ctx, pr.Issue, doer); err != nil {
		log.Error("ApplyCodeSuggestions: check resolve permission after commit %s: %v", result.Commit.SHA, err)
	} else if canResolve {
		for _, conversation := range changes.Conversations {
			if err := issues_model.MarkConversation(ctx, conversation, doer, true); err != nil {
				log.Error("ApplyCodeSuggestions: resolve conversation %d after commit %s: %v", conversation.ID, result.Commit.SHA, err)
			}
		}
	}
	return nil
}

func codeSuggestionsCommitMessage(doer *user_model.User, message, description string, comments issues_model.CommentList) string {
	if message = strings.TrimSpace(message); message == "" {
		message = pull_service.CodeSuggestionsBatchCommitMessage
		if len(comments) == 1 {
			message = comments[0].CodeSuggestionCommitMessage()
		}
	}
	if description = strings.TrimSpace(description); description != "" {
		message += "\n\n" + description
	}
	if !setting.Repository.PullRequest.AddCoCommitterTrailers {
		return message
	}
	coAuthors := container.SetOf(doer.ID)
	for _, comment := range comments {
		if comment.Poster.ID > 0 && coAuthors.Add(comment.Poster.ID) {
			message = pull_service.AddCommitMessageTailer(message, git.CoAuthoredByTrailer, comment.Poster.NewGitSig().String())
		}
	}
	return message
}
