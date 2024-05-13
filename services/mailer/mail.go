// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"mime"
	"regexp"
	"strconv"
	"strings"
	texttmpl "text/template"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/emoji"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/translation"
	incoming_payload "code.gitea.io/gitea/services/mailer/incoming/payload"
	"code.gitea.io/gitea/services/mailer/token"

	"gopkg.in/gomail.v2"
)

const (
	mailAuthActivate       base.TplName = "auth/activate"
	mailAuthActivateEmail  base.TplName = "auth/activate_email"
	mailAuthResetPassword  base.TplName = "auth/reset_passwd"
	mailAuthRegisterNotify base.TplName = "auth/register_notify"

	mailNotifyCollaborator base.TplName = "notify/collaborator"

	mailRepoTransferNotify base.TplName = "notify/repo_transfer"

	// There's no actual limit for subject in RFC 5322
	mailMaxSubjectRunes = 256
)

var (
	bodyTemplates       *template.Template
	subjectTemplates    *texttmpl.Template
	subjectRemoveSpaces = regexp.MustCompile(`[\s]+`)
)

// SendTestMail sends a test mail
func SendTestMail(email string) error {
	if setting.MailService == nil {
		// No mail service configured
		return nil
	}
	return gomail.Send(Sender, NewMessage(email, "Gitea Test Email!", "Gitea Test Email!").ToMessage())
}

// sendUserMail sends a mail to the user
func sendUserMail(language string, u *user_model.User, tpl base.TplName, code, subject, info string) {
	locale := translation.NewLocale(language)
	data := map[string]any{
		"locale":            locale,
		"DisplayName":       u.DisplayName(),
		"ActiveCodeLives":   timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, locale),
		"ResetPwdCodeLives": timeutil.MinutesToFriendly(setting.Service.ResetPwdCodeLives, locale),
		"Code":              code,
		"Language":          locale.Language(),
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(tpl), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := NewMessage(u.Email, subject, content.String())
	msg.Info = fmt.Sprintf("UID: %d, %s", u.ID, info)

	SendAsync(msg)
}

// SendActivateAccountMail sends an activation mail to the user (new user registration)
func SendActivateAccountMail(locale translation.Locale, u *user_model.User) {
	if setting.MailService == nil {
		// No mail service configured
		return
	}
	sendUserMail(locale.Language(), u, mailAuthActivate, u.GenerateEmailActivateCode(u.Email), locale.TrString("mail.activate_account"), "activate account")
}

// SendResetPasswordMail sends a password reset mail to the user
func SendResetPasswordMail(u *user_model.User) {
	if setting.MailService == nil {
		// No mail service configured
		return
	}
	locale := translation.NewLocale(u.Language)
	sendUserMail(u.Language, u, mailAuthResetPassword, u.GenerateEmailActivateCode(u.Email), locale.TrString("mail.reset_password"), "recover account")
}

// SendActivateEmailMail sends confirmation email to confirm new email address
func SendActivateEmailMail(u *user_model.User, email string) {
	if setting.MailService == nil {
		// No mail service configured
		return
	}
	locale := translation.NewLocale(u.Language)
	data := map[string]any{
		"locale":          locale,
		"DisplayName":     u.DisplayName(),
		"ActiveCodeLives": timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, locale),
		"Code":            u.GenerateEmailActivateCode(email),
		"Email":           email,
		"Language":        locale.Language(),
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailAuthActivateEmail), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := NewMessage(email, locale.TrString("mail.activate_email"), content.String())
	msg.Info = fmt.Sprintf("UID: %d, activate email", u.ID)

	SendAsync(msg)
}

// SendRegisterNotifyMail triggers a notify e-mail by admin created a account.
func SendRegisterNotifyMail(u *user_model.User) {
	if setting.MailService == nil || !u.IsActive {
		// No mail service configured OR user is inactive
		return
	}
	locale := translation.NewLocale(u.Language)

	data := map[string]any{
		"locale":      locale,
		"DisplayName": u.DisplayName(),
		"Username":    u.Name,
		"Language":    locale.Language(),
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailAuthRegisterNotify), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := NewMessage(u.Email, locale.TrString("mail.register_notify"), content.String())
	msg.Info = fmt.Sprintf("UID: %d, registration notify", u.ID)

	SendAsync(msg)
}

// SendCollaboratorMail sends mail notification to new collaborator.
func SendCollaboratorMail(u, doer *user_model.User, repo *repo_model.Repository) {
	if setting.MailService == nil || !u.IsActive {
		// No mail service configured OR the user is inactive
		return
	}
	locale := translation.NewLocale(u.Language)
	repoName := repo.FullName()

	subject := locale.TrString("mail.repo.collaborator.added.subject", doer.DisplayName(), repoName)
	data := map[string]any{
		"locale":   locale,
		"Subject":  subject,
		"RepoName": repoName,
		"Link":     repo.HTMLURL(),
		"Language": locale.Language(),
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailNotifyCollaborator), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := NewMessage(u.Email, subject, content.String())
	msg.Info = fmt.Sprintf("UID: %d, add collaborator", u.ID)

	SendAsync(msg)
}

func composeIssueCommentMessages(ctx *mailCommentContext, lang string, recipients []*user_model.User, fromMention bool, info string) ([]*Message, error) {
	var (
		subject string
		link    string
		prefix  string
		// Fall back subject for bad templates, make sure subject is never empty
		fallback       string
		reviewComments []*issues_model.Comment
	)

	commentType := issues_model.CommentTypeComment
	if ctx.Comment != nil {
		commentType = ctx.Comment.Type
		link = ctx.Issue.HTMLURL() + "#" + ctx.Comment.HashTag()
	} else {
		link = ctx.Issue.HTMLURL()
	}

	reviewType := issues_model.ReviewTypeComment
	if ctx.Comment != nil && ctx.Comment.Review != nil {
		reviewType = ctx.Comment.Review.Type
	}

	// This is the body of the new issue or comment, not the mail body
	body, err := markdown.RenderString(&markup.RenderContext{
		Ctx: ctx,
		Links: markup.Links{
			AbsolutePrefix: true,
			Base:           ctx.Issue.Repo.HTMLURL(),
		},
		Metas: ctx.Issue.Repo.ComposeMetas(ctx),
	}, ctx.Content)
	if err != nil {
		return nil, err
	}

	actType, actName, tplName := actionToTemplate(ctx.Issue, ctx.ActionType, commentType, reviewType)

	if actName != "new" {
		prefix = "Re: "
	}
	fallback = prefix + fallbackMailSubject(ctx.Issue)

	if ctx.Comment != nil && ctx.Comment.Review != nil {
		reviewComments = make([]*issues_model.Comment, 0, 10)
		for _, lines := range ctx.Comment.Review.CodeComments {
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
		"Issue":           ctx.Issue,
		"Comment":         ctx.Comment,
		"IsPull":          ctx.Issue.IsPull,
		"User":            ctx.Issue.Repo.MustOwner(ctx),
		"Repo":            ctx.Issue.Repo.FullName(),
		"Doer":            ctx.Doer,
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
	msgID := generateMessageIDForIssue(ctx.Issue, ctx.Comment, ctx.ActionType)
	reference := generateMessageIDForIssue(ctx.Issue, nil, activities_model.ActionType(0))

	var replyPayload []byte
	if ctx.Comment != nil {
		if ctx.Comment.Type.HasMailReplySupport() {
			replyPayload, err = incoming_payload.CreateReferencePayload(ctx.Comment)
		}
	} else {
		replyPayload, err = incoming_payload.CreateReferencePayload(ctx.Issue)
	}
	if err != nil {
		return nil, err
	}

	unsubscribePayload, err := incoming_payload.CreateReferencePayload(ctx.Issue)
	if err != nil {
		return nil, err
	}

	msgs := make([]*Message, 0, len(recipients))
	for _, recipient := range recipients {
		msg := NewMessageFrom(
			recipient.Email,
			ctx.Doer.GetCompleteName(),
			setting.MailService.FromEmail,
			subject,
			mailBody.String(),
		)
		msg.Info = fmt.Sprintf("Subject: %s, %s", subject, info)

		msg.SetHeader("Message-ID", msgID)
		msg.SetHeader("In-Reply-To", reference)

		references := []string{reference}
		listUnsubscribe := []string{"<" + ctx.Issue.HTMLURL() + ">"}

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

		for key, value := range generateAdditionalHeaders(ctx, actType, recipient) {
			msg.SetHeader(key, value)
		}

		msgs = append(msgs, msg)
	}

	return msgs, nil
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

func generateMessageIDForRelease(release *repo_model.Release) string {
	return fmt.Sprintf("<%s/releases/%d@%s>", release.Repo.FullName(), release.ID, setting.Domain)
}

func generateAdditionalHeaders(ctx *mailCommentContext, reason string, recipient *user_model.User) map[string]string {
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

func sanitizeSubject(subject string) string {
	runes := []rune(strings.TrimSpace(subjectRemoveSpaces.ReplaceAllLiteralString(subject, " ")))
	if len(runes) > mailMaxSubjectRunes {
		runes = runes[:mailMaxSubjectRunes]
	}
	// Encode non-ASCII characters
	return mime.QEncoding.Encode("utf-8", string(runes))
}

// SendIssueAssignedMail composes and sends issue assigned email
func SendIssueAssignedMail(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, content string, comment *issues_model.Comment, recipients []*user_model.User) error {
	if setting.MailService == nil {
		// No mail service configured
		return nil
	}

	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("Unable to load repo [%d] for issue #%d [%d]. Error: %v", issue.RepoID, issue.Index, issue.ID, err)
		return err
	}

	langMap := make(map[string][]*user_model.User)
	for _, user := range recipients {
		if !user.IsActive {
			// don't send emails to inactive users
			continue
		}
		langMap[user.Language] = append(langMap[user.Language], user)
	}

	for lang, tos := range langMap {
		msgs, err := composeIssueCommentMessages(&mailCommentContext{
			Context:    ctx,
			Issue:      issue,
			Doer:       doer,
			ActionType: activities_model.ActionType(0),
			Content:    content,
			Comment:    comment,
		}, lang, tos, false, "issue assigned")
		if err != nil {
			return err
		}
		SendAsync(msgs...)
	}
	return nil
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
