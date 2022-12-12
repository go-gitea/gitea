// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package incoming

import (
	"bytes"
	"context"
	"fmt"

	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/upload"
	attachment_service "code.gitea.io/gitea/services/attachment"
	issue_service "code.gitea.io/gitea/services/issue"
	incoming_payload "code.gitea.io/gitea/services/mailer/incoming/payload"
	"code.gitea.io/gitea/services/mailer/token"
	pull_service "code.gitea.io/gitea/services/pull"
)

type MailHandler interface {
	Handle(ctx context.Context, content *MailContent, user *user_model.User, payload []byte) error
}

var handlers = map[token.HandlerType]MailHandler{
	token.ReplyHandlerType:       &ReplyHandler{},
	token.UnsubscribeHandlerType: &UnsubscribeHandler{},
}

// ReplyHandler handles incoming emails to create a reply from them
type ReplyHandler struct{}

func (h *ReplyHandler) Handle(ctx context.Context, content *MailContent, user *user_model.User, payload []byte) error {
	if user == nil {
		return fmt.Errorf("user needed")
	}

	if content.Content == "" && len(content.Attachments) == 0 {
		return nil
	}

	ref, err := incoming_payload.GetReferenceFromPayload(ctx, payload)
	if err != nil {
		return err
	}

	var issue *issues_model.Issue

	switch r := ref.(type) {
	case *issues_model.Issue:
		issue = r
	case *issues_model.Comment:
		comment := r

		if err := comment.LoadIssue(ctx); err != nil {
			return err
		}

		issue = comment.Issue
	default:
		return fmt.Errorf("unsupported reply reference: %v", ref)
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}

	perm, err := access_model.GetUserRepoPermission(ctx, issue.Repo, user)
	if err != nil {
		return err
	}

	if !perm.CanWriteIssuesOrPulls(issue.IsPull) || issue.IsLocked && !user.IsAdmin {
		log.Debug("can't write issue or pull")
		return nil
	}

	switch r := ref.(type) {
	case *issues_model.Issue:
		attachmentIDs := make([]string, 0, len(content.Attachments))
		if setting.Attachment.Enabled {
			for _, attachment := range content.Attachments {
				a, err := attachment_service.UploadAttachment(bytes.NewReader(attachment.Content), setting.Attachment.AllowedTypes, &repo_model.Attachment{
					Name:       attachment.Name,
					UploaderID: user.ID,
					RepoID:     issue.Repo.ID,
				})
				if err != nil {
					if upload.IsErrFileTypeForbidden(err) {
						log.Debug("Skipping disallowed attachment type")
						continue
					}
					return err
				}
				attachmentIDs = append(attachmentIDs, a.UUID)
			}
		}

		_, err = issue_service.CreateIssueComment(ctx, user, issue.Repo, issue, content.Content, attachmentIDs)
		if err != nil {
			return fmt.Errorf("CreateIssueComment failed: %w", err)
		}
	case *issues_model.Comment:
		comment := r

		if comment.Type == issues_model.CommentTypeCode {
			_, err := pull_service.CreateCodeComment(
				ctx,
				user,
				nil,
				issue,
				comment.Line,
				content.Content,
				comment.TreePath,
				false,
				comment.ReviewID,
				"",
			)
			if err != nil {
				return fmt.Errorf("CreateCodeComment failed: %w", err)
			}
		}
	}
	return nil
}

// UnsubscribeHandler handles unwatching issues/pulls
type UnsubscribeHandler struct{}

func (h *UnsubscribeHandler) Handle(ctx context.Context, _ *MailContent, user *user_model.User, payload []byte) error {
	if user == nil {
		return fmt.Errorf("user needed")
	}

	ref, err := incoming_payload.GetReferenceFromPayload(ctx, payload)
	if err != nil {
		return err
	}

	switch r := ref.(type) {
	case *issues_model.Issue:
		issue := r

		if err := issue.LoadRepo(ctx); err != nil {
			return err
		}

		perm, err := access_model.GetUserRepoPermission(ctx, issue.Repo, user)
		if err != nil {
			return err
		}

		if !perm.CanReadIssuesOrPulls(issue.IsPull) {
			log.Debug("can't read issue or pull")
			return nil
		}

		return issues_model.CreateOrUpdateIssueWatch(user.ID, issue.ID, false)
	}

	return fmt.Errorf("unsupported unsubscribe reference: %v", ref)
}
