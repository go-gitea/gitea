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
func AddTeamMember(ctx context.Context, org *org_model.Organization, inviter *user_model.User, team *org_model.Team, username string, locale translation.Locale) error {
	uname := utils.RemoveUsernameParameterSuffix(strings.ToLower(username))
	var err error
	var u *user_model.User
	if u, err = user_model.GetUserByName(ctx, uname); err != nil {
		if user_model.IsErrUserNotExist(err) {
			if setting.MailService != nil && user_model.ValidateEmail(uname) == nil {
				if err := CreateTeamInvite(ctx, inviter, team, uname); err != nil {
					if org_model.IsErrTeamInviteAlreadyExist(err) {
						return errors.New(locale.Tr("form.duplicate_invite_to_team"))
					} else if org_model.IsErrUserEmailAlreadyAdded(err) {
						return errors.New(locale.Tr("org.teams.add_duplicate_users"))
					}
					return err
				}
			} else {
				err = errors.New(locale.Tr("form.user_not_exist"))
			}
		}
		return err
	}

	if u.IsOrganization() {
		return errors.New(locale.Tr("form.cannot_add_org_to_team"))
	}

	if team.IsMember(ctx, u.ID) {
		err = errors.New(locale.Tr("org.teams.add_duplicate_users"))
	} else {
		err = models.AddTeamMember(ctx, team, u.ID)
	}
	if err != nil {
		if org_model.IsErrLastOrgOwner(err) {
			err = errors.New(locale.Tr("form.last_org_owner"))
		} else {
			log.Error("Action(%s): %v", "add", err)
		}
	}
	return err
}
