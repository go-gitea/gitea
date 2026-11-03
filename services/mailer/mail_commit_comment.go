// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"slices"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/base"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/translation"
	"gitea.dev/modules/util"
	sender_service "gitea.dev/services/mailer/sender"
)

func MailCommitComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	commit *git.Commit, comment *issues_model.Comment, mentions []*user_model.User,
) error {
	if setting.MailService == nil {
		return nil
	}
	participantIDs, err := issues_model.GetCommitCommentPosterIDs(ctx, repo.ID, commit.ID.String())
	if err != nil {
		return err
	}
	if author := user_model.GetUserByGitAuthor(ctx, commit); author != nil {
		participantIDs = append(participantIDs, author.ID)
	}
	watcherIDs, err := repo_model.GetRepoAllActivityWatchersIDs(ctx, repo.ID)
	if err != nil {
		return err
	}
	participantIDs = append(participantIDs, watcherIDs...)
	ignorers, err := repo_model.GetRepoIgnorersIDs(ctx, repo.ID)
	if err != nil {
		return err
	}
	visited := container.SetOf(ignorers...)
	if doer.EmailNotificationsPreference != user_model.EmailNotificationsAndYourOwn {
		visited.Add(doer.ID)
	}
	for _, fromMention := range []bool{true, false} {
		recipients := mentions
		if !fromMention {
			recipients, err = user_model.GetMailableUsersByIDs(ctx, slices.DeleteFunc(participantIDs, func(id int64) bool { return visited.Contains(id) }), false)
			if err != nil {
				return err
			}
		}
		if err := mailCommentBatch(ctx, repo, unit.TypeCode, recipients, visited, fromMention, nil,
			func(lang string, recipients []*user_model.User) ([]*sender_service.Message, error) {
				return composeCommitCommentMessages(ctx, doer, repo, commit, comment, lang, recipients, fromMention)
			}); err != nil {
			return err
		}
	}
	return nil
}

func composeCommitCommentMessages(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	commit *git.Commit, comment *issues_model.Comment, lang string, recipients []*user_model.User, fromMention bool,
) ([]*sender_service.Message, error) {
	body, err := renderMailContent(ctx, doer, repo, comment.Content)
	if err != nil {
		return nil, err
	}
	locale := translation.NewLocale(lang)
	shortSHA := base.ShortSha(commit.ID.String())
	subject := sanitizeSubject(locale.TrString("mail.commit_comment.subject", repo.FullName(), shortSHA))
	commitURL := repo.HTMLURL() + "/commit/" + commit.ID.String()
	var mailBody bytes.Buffer
	if err := LoadedTemplates().BodyTemplates.ExecuteTemplate(&mailBody, "mail/repo/commit_comment", map[string]any{
		"locale":    locale,
		"Subject":   subject,
		"Body":      body,
		"Doer":      doer,
		"IsMention": fromMention,
		"Link":      commitURL + "#" + comment.HashTag(),
		"Location":  util.Iif(comment.TreePath == "", shortSHA, fmt.Sprintf("%s:%d", comment.TreePath, comment.UnsignedLine())),
		"CommitURL": commitURL,
		"ShortSHA":  shortSHA,
	}); err != nil {
		return nil, err
	}
	reference := fmt.Sprintf("<%s/commit/%s@%s>", repo.FullName(), commit.ID.String(), setting.AppDomain)
	headers := generateMetadataHeaders(repo)
	maps.Copy(headers, generateReasonHeaders(util.Iif(fromMention, "mention", "commit")))
	msgs := make([]*sender_service.Message, 0, len(recipients))
	for _, recipient := range recipients {
		msg := sender_service.NewMessageFrom(recipient.Email, fromDisplayName(doer), setting.MailService.FromEmail, subject, mailBody.String())
		msg.Info = fmt.Sprintf("Subject: %s, commit comments", subject)
		msg.SetHeader("Message-ID", fmt.Sprintf("<%s/commit/%s/comment/%d@%s>", repo.FullName(), commit.ID.String(), comment.ID, setting.AppDomain))
		msg.SetHeader("In-Reply-To", reference)
		msg.SetHeader("References", reference)
		maps.Copy(headers, generateSenderRecipientHeaders(doer, recipient))
		for key, value := range headers {
			msg.SetHeader(key, value)
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}
