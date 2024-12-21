// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"fmt"
	"net/url"

	org_model "code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
	sender_service "code.gitea.io/gitea/services/mailer/sender"
)

const (
	tplTeamInviteMail templates.TplName = "team_invite"
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

	// check if a user with this email already exists
	user, err := user_model.GetUserByEmail(ctx, invite.Email)
	if err != nil && !user_model.IsErrUserNotExist(err) {
		return err
	} else if user != nil && user.ProhibitLogin {
		return fmt.Errorf("login is prohibited for the invited user")
	}

	inviteRedirect := url.QueryEscape(fmt.Sprintf("/org/invite/%s", invite.Token))
	inviteURL := fmt.Sprintf("%suser/sign_up?redirect_to=%s", setting.AppURL, inviteRedirect)

	if (err == nil && user != nil) || setting.Service.DisableRegistration || setting.Service.AllowOnlyExternalRegistration {
		// user account exists or registration disabled
		inviteURL = fmt.Sprintf("%suser/login?redirect_to=%s", setting.AppURL, inviteRedirect)
	}

	subject := locale.TrString("mail.team_invite.subject", inviter.DisplayName(), org.DisplayName())
	mailMeta := map[string]any{
		"locale":       locale,
		"Inviter":      inviter,
		"Organization": org,
		"Team":         team,
		"Invite":       invite,
		"Subject":      subject,
		"InviteURL":    inviteURL,
	}

	var mailBody bytes.Buffer
	if err := bodyTemplates.ExecuteTemplate(&mailBody, string(tplTeamInviteMail), mailMeta); err != nil {
		log.Error("ExecuteTemplate [%s]: %v", string(tplTeamInviteMail)+"/body", err)
		return err
	}

	msg := sender_service.NewMessage(invite.Email, subject, mailBody.String())
	msg.Info = subject

	SendAsync(msg)

	return nil
}
