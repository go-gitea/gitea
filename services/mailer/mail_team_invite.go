// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"

	org_model "code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
)

const (
	tplTeamInviteMail base.TplName = "team_invite"
)

// MailTeamInvite sends team invites
func MailTeamInvite(ctx context.Context, inviter *user_model.User, team *org_model.Team, invite *org_model.TeamInvite) error {
	if setting.MailService == nil {
		return nil
	}

	org, err := user_model.GetUserByID(ctx, team.OrgID)
	if err != nil {
		return err
	}

	locale := translation.NewLocale(inviter.Language)

	subject := locale.Tr("mail.team_invite.subject", inviter.DisplayName(), org.DisplayName())
	mailMeta := map[string]any{
		"Inviter":      inviter,
		"Organization": org,
		"Team":         team,
		"Invite":       invite,
		"Subject":      subject,
		// helper
		"locale":    locale,
		"Str2html":  templates.Str2html,
		"DotEscape": templates.DotEscape,
	}

	var mailBody bytes.Buffer
	if err := bodyTemplates.ExecuteTemplate(&mailBody, string(tplTeamInviteMail), mailMeta); err != nil {
		log.Error("ExecuteTemplate [%s]: %v", string(tplTeamInviteMail)+"/body", err)
		return err
	}

	msg := NewMessage(invite.Email, subject, mailBody.String())
	msg.Info = subject

	SendAsync(msg)

	return nil
}
