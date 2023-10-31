// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"context"
	"errors"
	"strings"

	"code.gitea.io/gitea/models"
	org_model "code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/routers/utils"
	"code.gitea.io/gitea/services/mailer"
)

// CreateTeamInvite make a persistent invite in db and mail it
func CreateTeamInvite(ctx context.Context, inviter *user_model.User, team *org_model.Team, uname string) error {
	invite, err := org_model.CreateTeamInvite(ctx, inviter, team, uname)
	if err != nil {
		return err
	}

	return mailer.MailTeamInvite(ctx, inviter, team, invite)
}

// AddTeamMember add user to a team
func AddTeamMember(ctx context.Context, inviter *user_model.User, team *org_model.Team, username string, locale translation.Locale) (isServerError bool, err error) {
	uname := utils.RemoveUsernameParameterSuffix(strings.ToLower(username))
	var u *user_model.User
	if u, err = user_model.GetUserByName(ctx, uname); err != nil {
		if user_model.IsErrUserNotExist(err) {
			if setting.MailService != nil && user_model.ValidateEmail(uname) == nil {
				if err := CreateTeamInvite(ctx, inviter, team, uname); err != nil {
					if org_model.IsErrTeamInviteAlreadyExist(err) {
						return false, errors.New(locale.Tr("form.duplicate_invite_to_team"))
					} else if org_model.IsErrUserEmailAlreadyAdded(err) {
						return false, errors.New(locale.Tr("org.teams.add_duplicate_users"))
					} else {
						log.Error("CreateTeamInvite: %v", err)
						return true, err
					}
				}
			} else {
				err = errors.New(locale.Tr("form.user_not_exist"))
			}
		} else {
			log.Error("GetUserByName: %v", err)
			return true, err
		}
		return false, err
	}

	if u.IsOrganization() {
		return false, errors.New(locale.Tr("form.cannot_add_org_to_team"))
	}

	if team.IsMember(ctx, u.ID) {
		err = errors.New(locale.Tr("org.teams.add_duplicate_users"))
	} else {
		if err = models.AddTeamMember(ctx, team, u.ID); err != nil {
			return true, err
		}
	}
	return false, err
}
