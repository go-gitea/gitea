// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/translation"
)

const (
	tplNewReleaseMail base.TplName = "release"
)

// MailNewRelease send new release notify to all repo watchers.
func MailNewRelease(ctx context.Context, rel *repo_model.Release) {
	if setting.MailService == nil {
		// No mail service configured
		return
	}

	watcherIDList, err := repo_model.GetRepoWatchersIDs(ctx, rel.RepoID)
	if err != nil {
		log.Error("GetRepoWatchersIDs(%d): %v", rel.RepoID, err)
		return
	}

	recipients, err := user_model.GetMaileableUsersByIDs(ctx, watcherIDList, false)
	if err != nil {
		log.Error("user_model.GetMaileableUsersByIDs: %v", err)
		return
	}

	langMap := make(map[string][]string)
	for _, user := range recipients {
		if user.ID != rel.PublisherID {
			langMap[user.Language] = append(langMap[user.Language], user.Email)
		}
	}

	for lang, tos := range langMap {
		mailNewRelease(ctx, lang, tos, rel)
	}
}

func mailNewRelease(ctx context.Context, lang string, tos []string, rel *repo_model.Release) {
	locale := translation.NewLocale(lang)

	var err error
	rel.RenderedNote, err = markdown.RenderString(&markup.RenderContext{
		Ctx:       ctx,
		URLPrefix: rel.Repo.Link(),
		Metas:     rel.Repo.ComposeMetas(ctx),
	}, rel.Note)
	if err != nil {
		log.Error("markdown.RenderString(%d): %v", rel.RepoID, err)
		return
	}

	subject := locale.Tr("mail.release.new.subject", rel.TagName, rel.Repo.FullName())
	mailMeta := map[string]any{
		"locale":   locale,
		"Release":  rel,
		"Subject":  subject,
		"Language": locale.Language(),
	}

	var mailBody bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&mailBody, string(tplNewReleaseMail), mailMeta); err != nil {
		log.Error("ExecuteTemplate [%s]: %v", string(tplNewReleaseMail)+"/body", err)
		return
	}

	msgs := make([]*Message, 0, len(tos))
	publisherName := rel.Publisher.DisplayName()
	relURL := "<" + rel.HTMLURL() + ">"
	for _, to := range tos {
		msg := NewMessageFrom(to, publisherName, setting.MailService.FromEmail, subject, mailBody.String())
		msg.Info = subject
		msg.SetHeader("Message-ID", relURL)
		msgs = append(msgs, msg)
	}

	SendAsync(msgs...)
}
