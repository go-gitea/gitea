// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	access_model "codenest.dev/models/perm/access"
	"codenest.dev/models/renderhelper"
	repo_model "codenest.dev/models/repo"
	"codenest.dev/models/unit"
	user_model "codenest.dev/models/user"
	"codenest.dev/modules/container"
	"codenest.dev/modules/log"
	"codenest.dev/modules/markup/markdown"
	"codenest.dev/modules/setting"
	"codenest.dev/modules/templates"
	"codenest.dev/modules/translation"
	sender_service "codenest.dev/services/mailer/sender"
)

const tplNewReleaseMail templates.TplName = "mail/repo/release"

func generateMessageIDForRelease(release *repo_model.Release) string {
	return fmt.Sprintf("<%s/releases/%d@%s>", release.Repo.FullName(), release.ID, setting.AppDomain)
}

// MailNewRelease send new release notify to all repo watchers.
func MailNewRelease(ctx context.Context, rel *repo_model.Release) {
	if setting.MailService == nil {
		// No mail service configured
		return
	}

	watcherIDList, err := repo_model.GetRepoWatchersIDs(ctx, rel.RepoID, repo_model.WatchReleases)
	if err != nil {
		log.Error("GetRepoWatchersIDs(%d): %v", rel.RepoID, err)
		return
	}

	customWatcherIDs, err := repo_model.GetRepoCustomWatchersIDs(ctx, rel.RepoID, repo_model.WatchReleases)
	if err != nil {
		log.Error("GetRepoCustomWatchersIDs(%d): %v", rel.RepoID, err)
		return
	}
	customWatchers := container.SetOf(customWatcherIDs...)

	recipients, err := user_model.GetMailableUsersByIDs(ctx, watcherIDList, len(customWatchers) > 0)
	if err != nil {
		log.Error("user_model.GetMailableUsersByIDs: %v", err)
		return
	}

	if err := rel.LoadRepo(ctx); err != nil {
		log.Error("rel.LoadRepo: %v", err)
		return
	}

	// delete publisher, on-mention users without a custom watch or any users with no permission
	recipients = slices.DeleteFunc(recipients, func(u *user_model.User) bool {
		return u.ID == rel.PublisherID ||
			u.EmailNotificationsPreference == user_model.EmailNotificationsOnMention && !customWatchers.Contains(u.ID) ||
			!access_model.CheckRepoUnitUser(ctx, rel.Repo, u, unit.TypeReleases)
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
