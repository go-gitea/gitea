// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"bytes"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplNewReleaseMail base.TplName = "release"
)

// MailNewRelease send new release notify to all all repo watchers.
func MailNewRelease(rel *models.Release) {
	watcherIDList, err := models.GetRepoWatchersIDs(rel.RepoID)
	if err != nil {
		log.Error("GetRepoWatchersIDs(%d): %v", rel.RepoID, err)
		return
	}

	recipients, err := models.GetMaileableUsersByIDs(watcherIDList, false)
	if err != nil {
		log.Error("models.GetMaileableUsersByIDs: %v", err)
		return
	}

	tos := make([]string, 0, len(recipients))
	for _, to := range recipients {
		if to.ID != rel.PublisherID {
			tos = append(tos, to.Email)
		}
	}

	rel.RenderedNote = markdown.RenderString(rel.Note, rel.Repo.Link(), rel.Repo.ComposeMetas())
	subject := fmt.Sprintf("%s in %s released", rel.TagName, rel.Repo.FullName())

	mailMeta := map[string]interface{}{
		"Release": rel,
		"Subject": subject,
	}

	var mailBody bytes.Buffer

	if err = bodyTemplates.ExecuteTemplate(&mailBody, string(tplNewReleaseMail), mailMeta); err != nil {
		log.Error("ExecuteTemplate [%s]: %v", string(tplNewReleaseMail)+"/body", err)
		return
	}

	msgs := make([]*Message, 0, len(recipients))
	publisherName := rel.Publisher.DisplayName()
	relURL := "<" + rel.HTMLURL() + ">"
	for _, to := range tos {
		msg := NewMessageFrom([]string{to}, publisherName, setting.MailService.FromEmail, subject, mailBody.String())
		msg.Info = subject
		msg.SetHeader("Message-ID", relURL)
		msgs = append(msgs, msg)
	}

	SendAsyncs(msgs)
}
