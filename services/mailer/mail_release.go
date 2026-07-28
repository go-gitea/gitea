// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	access_model "gitea.dev/models/perm/access"
	"gitea.dev/models/renderhelper"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/translation"
	sender_service "gitea.dev/services/mailer/sender"
)

const tplNewReleaseMail templates.TplName = "repo/release"

func generateMessageIDForRelease(release *repo_model.Release) string {
	return fmt.Sprintf("<%s/releases/%d@%s>", release.Repo.FullName(), release.ID, setting.Domain)
}

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

	recipients, err := user_model.GetMailableUsersByIDs(ctx, watcherIDList, false)
	if err != nil {
		log.Error("user_model.GetMailableUsersByIDs: %v", err)
		return
	}

	if err := rel.LoadRepo(ctx); err != nil {
		log.Error("rel.LoadRepo: %v", err)
		return
	}

	// delete publisher or any users with no permission
	recipients = slices.DeleteFunc(recipients, func(u *user_model.User) bool {
		return u.ID == rel.PublisherID || !access_model.CheckRepoUnitUser(ctx, rel.Repo, u, unit.TypeReleases)
	})

	langMap := make(map[string][]*user_model.User)
	for _, user := range recipients {
		if user.ID != rel.PublisherID {
			langMap[user.Language] = append(langMap[user.Language], user)
		}
	}

	for lang, tos := range langMap {
		mailNewRelease(ctx, lang, tos, rel)
	}
}

func mailNewRelease(ctx context.Context, lang string, tos []*user_model.User, rel *repo_model.Release) {
	locale := translation.NewLocale(lang)

	var err error
	rctx := renderhelper.NewRenderContextRepoComment(ctx, rel.Repo).WithUseAbsoluteLink(true)
	rel.RenderedNote, err = markdown.RenderString(rctx,
		rel.Note)
	if err != nil {
		log.Error("markdown.RenderString(%d): %v", rel.RepoID, err)
		return
	}

	subject := locale.TrString("mail.release.new.subject", rel.TagName, rel.Repo.FullName())
	mailMeta := map[string]any{
		"locale":   locale,
		"Release":  rel,
		"Subject":  subject,
		"Language": locale.Language(),
		"Link":     rel.HTMLURL(),
	}

	var mailBody bytes.Buffer

	if err := LoadedTemplates().BodyTemplates.ExecuteTemplate(&mailBody, string(tplNewReleaseMail), mailMeta); err != nil {
		log.Error("ExecuteTemplate [%s]: %v", string(tplNewReleaseMail)+"/body", err)
		return
	}

	msgs := make([]*sender_service.Message, 0, len(tos))
	publisherName := fromDisplayName(rel.Publisher)
	msgID := generateMessageIDForRelease(rel)
	for _, to := range tos {
		msg := sender_service.NewMessageFrom(to.EmailTo(), publisherName, setting.MailService.FromEmail, subject, mailBody.String())
		msg.Info = subject
		msg.SetHeader("Message-ID", msgID)
		msgs = append(msgs, msg)
	}

	SendAsync(msgs...)
}
