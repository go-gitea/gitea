// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/renderhelper"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/emoji"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/translation"
	incoming_payload "code.gitea.io/gitea/services/mailer/incoming/payload"
	sender_service "code.gitea.io/gitea/services/mailer/sender"
	"code.gitea.io/gitea/services/mailer/token"
)

// maxEmailBodySize is the approximate maximum size of an email body in bytes
// Many e-mail service providers have limitations on the size of the email body, it's usually from 10MB to 25MB
const maxEmailBodySize = 9_000_000

func fallbackMailSubject(issue *issues_model.Issue) string {
	return fmt.Sprintf("[%s] %s (#%d)", issue.Repo.FullName(), issue.Title, issue.Index)
}

type mailComment struct {
	Issue                 *issues_model.Issue
	Doer                  *user_model.User
	ActionType            activities_model.ActionType
	Content               string
	Comment               *issues_model.Comment
	ForceDoerNotification bool
}

func composeIssueCommentMessages(ctx context.Context, comment *mailComment, lang string, recipients []*user_model.User, fromMention bool, info string) ([]*sender_service.Message, error) {
	var (
		subject string
		link    string
		prefix  string
		// Fall back subject for bad templates, make sure subject is never empty
		fallback       string
		reviewComments []*issues_model.Comment
	)

	commentType := issues_model.CommentTypeComment
	if comment.Comment != nil {
		commentType = comment.Comment.Type
		link = comment.Issue.HTMLURL() + "#" + comment.Comment.HashTag()
	} else {
		link = comment.Issue.HTMLURL()
	}

	reviewType := issues_model.ReviewTypeComment
	if comment.Comment != nil && comment.Comment.Review != nil {
		reviewType = comment.Comment.Review.Type
	}

	// This is the body of the new issue or comment, not the mail body
	rctx := renderhelper.NewRenderContextRepoComment(ctx, comment.Issue.Repo).WithUseAbsoluteLink(true)
	body, err := markdown.RenderString(rctx, comment.Content)
	if err != nil {
		return nil, err
	}

	if setting.MailService.EmbedAttachmentImages {
		attEmbedder := newMailAttachmentBase64Embedder(comment.Doer, comment.Issue.Repo, maxEmailBodySize)
		bodyAfterEmbedding, err := attEmbedder.Base64InlineImages(ctx, body)
		if err != nil {
			log.Error("Failed to embed images in mail body: %v", err)
		} else {
			body = bodyAfterEmbedding
		}
	}
	actType, actName, tplName := actionToTemplate(comment.Issue, comment.ActionType, commentType, reviewType)

	if actName != "new" {
		prefix = "Re: "
	}
	fallback = prefix + fallbackMailSubject(comment.Issue)

	if comment.Comment != nil && comment.Comment.Review != nil {
		reviewComments = make([]*issues_model.Comment, 0, 10)
		for _, lines := range comment.Comment.Review.CodeComments {
			for _, comments := range lines {
				reviewComments = append(reviewComments, comments...)
			}
		}
	}
	locale := translation.NewLocale(lang)

	mailMeta := map[string]any{
		"locale":          locale,
		"FallbackSubject": fallback,
		"Body":            body,
		"Link":            link,
		"Issue":           comment.Issue,
		"Comment":         comment.Comment,
		"IsPull":          comment.Issue.IsPull,
		"User":            comment.Issue.Repo.MustOwner(ctx),
		"Repo":            comment.Issue.Repo.FullName(),
		"Doer":            comment.Doer,
		"IsMention":       fromMention,
		"SubjectPrefix":   prefix,
		"ActionType":      actType,
		"ActionName":      actName,
		"ReviewComments":  reviewComments,
		"Language":        locale.Language(),
		"CanReply":        setting.IncomingEmail.Enabled && commentType != issues_model.CommentTypePullRequestPush,
	}

	var mailSubject bytes.Buffer
	if err := subjectTemplates.ExecuteTemplate(&mailSubject, tplName, mailMeta); err == nil {
		subject = sanitizeSubject(mailSubject.String())
		if subject == "" {
			subject = fallback
		}
	} else {
		log.Error("ExecuteTemplate [%s]: %v", tplName+"/subject", err)
	}

	subject = emoji.ReplaceAliases(subject)

	mailMeta["Subject"] = subject

	var mailBody bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&mailBody, tplName, mailMeta); err != nil {
		log.Error("ExecuteTemplate [%s]: %v", tplName+"/body", err)
	}

	// Make sure to compose independent messages to avoid leaking user emails
	msgID := generateMessageIDForIssue(comment.Issue, comment.Comment, comment.ActionType)
	reference := generateMessageIDForIssue(comment.Issue, nil, activities_model.ActionType(0))

	var replyPayload []byte
	if comment.Comment != nil {
		if comment.Comment.Type.HasMailReplySupport() {
			replyPayload, err = incoming_payload.CreateReferencePayload(comment.Comment)
		}
	} else {
		replyPayload, err = incoming_payload.CreateReferencePayload(comment.Issue)
	}
	if err != nil {
		return nil, err
	}

	unsubscribePayload, err := incoming_payload.CreateReferencePayload(comment.Issue)
	if err != nil {
		return nil, err
	}

	msgs := make([]*sender_service.Message, 0, len(recipients))
	for _, recipient := range recipients {
		msg := sender_service.NewMessageFrom(
			recipient.Email,
			fromDisplayName(comment.Doer),
			setting.MailService.FromEmail,
			subject,
			mailBody.String(),
		)
		msg.Info = fmt.Sprintf("Subject: %s, %s", subject, info)

		msg.SetHeader("Message-ID", msgID)
		msg.SetHeader("In-Reply-To", reference)

		references := []string{reference}
		listUnsubscribe := []string{"<" + comment.Issue.HTMLURL() + ">"}

		if setting.IncomingEmail.Enabled {
			if replyPayload != nil {
				token, err := token.CreateToken(token.ReplyHandlerType, recipient, replyPayload)
				if err != nil {
					log.Error("CreateToken failed: %v", err)
				} else {
					replyAddress := strings.Replace(setting.IncomingEmail.ReplyToAddress, setting.IncomingEmail.TokenPlaceholder, token, 1)
					msg.ReplyTo = replyAddress
					msg.SetHeader("List-Post", fmt.Sprintf("<mailto:%s>", replyAddress))

					references = append(references, fmt.Sprintf("<reply-%s@%s>", token, setting.Domain))
				}
			}

			token, err := token.CreateToken(token.UnsubscribeHandlerType, recipient, unsubscribePayload)
			if err != nil {
				log.Error("CreateToken failed: %v", err)
			} else {
				unsubAddress := strings.Replace(setting.IncomingEmail.ReplyToAddress, setting.IncomingEmail.TokenPlaceholder, token, 1)
				listUnsubscribe = append(listUnsubscribe, "<mailto:"+unsubAddress+">")
			}
		}

		msg.SetHeader("References", references...)
		msg.SetHeader("List-Unsubscribe", listUnsubscribe...)

		for key, value := range generateAdditionalHeaders(comment, actType, recipient) {
			msg.SetHeader(key, value)
		}

		msgs = append(msgs, msg)
	}

	return msgs, nil
}

// actionToTemplate returns the type and name of the action facing the user
// (slightly different from activities_model.ActionType) and the name of the template to use (based on availability)
func actionToTemplate(issue *issues_model.Issue, actionType activities_model.ActionType,
	commentType issues_model.CommentType, reviewType issues_model.ReviewType,
) (typeName, name, template string) {
	if issue.IsPull {
		typeName = "pull"
	} else {
		typeName = "issue"
	}
	switch actionType {
	case activities_model.ActionCreateIssue, activities_model.ActionCreatePullRequest:
		name = "new"
	case activities_model.ActionCommentIssue, activities_model.ActionCommentPull:
		name = "comment"
	case activities_model.ActionCloseIssue, activities_model.ActionClosePullRequest:
		name = "close"
	case activities_model.ActionReopenIssue, activities_model.ActionReopenPullRequest:
		name = "reopen"
	case activities_model.ActionMergePullRequest, activities_model.ActionAutoMergePullRequest:
		name = "merge"
	case activities_model.ActionPullReviewDismissed:
		name = "review_dismissed"
	case activities_model.ActionPullRequestReadyForReview:
		name = "ready_for_review"
	default:
		switch commentType {
		case issues_model.CommentTypeReview:
			switch reviewType {
			case issues_model.ReviewTypeApprove:
				name = "approve"
			case issues_model.ReviewTypeReject:
				name = "reject"
			default:
				name = "review"
			}
		case issues_model.CommentTypeCode:
			name = "code"
		case issues_model.CommentTypeAssignees:
			name = "assigned"
		case issues_model.CommentTypePullRequestPush:
			name = "push"
		default:
			name = "default"
		}
	}

	template = typeName + "/" + name
	ok := bodyTemplates.Lookup(template) != nil
	if !ok && typeName != "issue" {
		template = "issue/" + name
		ok = bodyTemplates.Lookup(template) != nil
	}
	if !ok {
		template = typeName + "/default"
		ok = bodyTemplates.Lookup(template) != nil
	}
	if !ok {
		template = "issue/default"
	}
	return typeName, name, template
}

func generateMessageIDForIssue(issue *issues_model.Issue, comment *issues_model.Comment, actionType activities_model.ActionType) string {
	var path string
	if issue.IsPull {
		path = "pulls"
	} else {
		path = "issues"
	}

	var extra string
	if comment != nil {
		extra = fmt.Sprintf("/comment/%d", comment.ID)
	} else {
		switch actionType {
		case activities_model.ActionCloseIssue, activities_model.ActionClosePullRequest:
			extra = fmt.Sprintf("/close/%d", time.Now().UnixNano()/1e6)
		case activities_model.ActionReopenIssue, activities_model.ActionReopenPullRequest:
			extra = fmt.Sprintf("/reopen/%d", time.Now().UnixNano()/1e6)
		case activities_model.ActionMergePullRequest, activities_model.ActionAutoMergePullRequest:
			extra = fmt.Sprintf("/merge/%d", time.Now().UnixNano()/1e6)
		case activities_model.ActionPullRequestReadyForReview:
			extra = fmt.Sprintf("/ready/%d", time.Now().UnixNano()/1e6)
		}
	}

	return fmt.Sprintf("<%s/%s/%d%s@%s>", issue.Repo.FullName(), path, issue.Index, extra, setting.Domain)
}

func generateAdditionalHeaders(ctx *mailComment, reason string, recipient *user_model.User) map[string]string {
	repo := ctx.Issue.Repo

	return map[string]string{
		// https://datatracker.ietf.org/doc/html/rfc2919
		"List-ID": fmt.Sprintf("%s <%s.%s.%s>", repo.FullName(), repo.Name, repo.OwnerName, setting.Domain),

		// https://datatracker.ietf.org/doc/html/rfc2369
		"List-Archive": fmt.Sprintf("<%s>", repo.HTMLURL()),

		"X-Mailer":                  "Gitea",
		"X-Gitea-Reason":            reason,
		"X-Gitea-Sender":            ctx.Doer.Name,
		"X-Gitea-Recipient":         recipient.Name,
		"X-Gitea-Recipient-Address": recipient.Email,
		"X-Gitea-Repository":        repo.Name,
		"X-Gitea-Repository-Path":   repo.FullName(),
		"X-Gitea-Repository-Link":   repo.HTMLURL(),
		"X-Gitea-Issue-ID":          strconv.FormatInt(ctx.Issue.Index, 10),
		"X-Gitea-Issue-Link":        ctx.Issue.HTMLURL(),

		"X-GitHub-Reason":            reason,
		"X-GitHub-Sender":            ctx.Doer.Name,
		"X-GitHub-Recipient":         recipient.Name,
		"X-GitHub-Recipient-Address": recipient.Email,

		"X-GitLab-NotificationReason": reason,
		"X-GitLab-Project":            repo.Name,
		"X-GitLab-Project-Path":       repo.FullName(),
		"X-GitLab-Issue-IID":          strconv.FormatInt(ctx.Issue.Index, 10),
	}
}
