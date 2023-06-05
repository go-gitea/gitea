// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

type ErrTeamInviteAlreadyExist struct {
	TeamID int64
	Email  string
}

func IsErrTeamInviteAlreadyExist(err error) bool {
	_, ok := err.(ErrTeamInviteAlreadyExist)
	return ok
}

func (err ErrTeamInviteAlreadyExist) Error() string {
	return fmt.Sprintf("team invite already exists [team_id: %d, email: %s]", err.TeamID, err.Email)
}

func (err ErrTeamInviteAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

type ErrTeamInviteNotFound struct {
	Token string
}

func IsErrTeamInviteNotFound(err error) bool {
	_, ok := err.(ErrTeamInviteNotFound)
	return ok
}

func (err ErrTeamInviteNotFound) Error() string {
	return fmt.Sprintf("team invite was not found [token: %s]", err.Token)
}

func (err ErrTeamInviteNotFound) Unwrap() error {
	return util.ErrNotExist
}

// ErrUserEmailAlreadyAdded represents a "user by email already added to team" error.
type ErrUserEmailAlreadyAdded struct {
	Email string
}

// IsErrUserEmailAlreadyAdded checks if an error is a ErrUserEmailAlreadyAdded.
func IsErrUserEmailAlreadyAdded(err error) bool {
	_, ok := err.(ErrUserEmailAlreadyAdded)
	return ok
}

func (err ErrUserEmailAlreadyAdded) Error() string {
	return fmt.Sprintf("user with email already added [email: %s]", err.Email)
}

func (err ErrUserEmailAlreadyAdded) Unwrap() error {
	return util.ErrAlreadyExist
}

// TeamInvite represents an invite to a team
type TeamInvite struct {
	ID          int64              `xorm:"pk autoincr"`
	Token       string             `xorm:"UNIQUE(token) INDEX NOT NULL DEFAULT ''"`
	InviterID   int64              `xorm:"NOT NULL DEFAULT 0"`
	OrgID       int64              `xorm:"INDEX NOT NULL DEFAULT 0"`
	TeamID      int64              `xorm:"UNIQUE(team_mail) INDEX NOT NULL DEFAULT 0"`
	Email       string             `xorm:"UNIQUE(team_mail) NOT NULL DEFAULT ''"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func CreateTeamInvite(ctx context.Context, doer *user_model.User, team *Team, email string) (*TeamInvite, error) {
	has, err := db.GetEngine(ctx).Exist(&TeamInvite{
		TeamID: team.ID,
		Email:  email,
	})
	if err != nil {
		return nil, err
	}
	if has {
		return nil, ErrTeamInviteAlreadyExist{
			TeamID: team.ID,
			Email:  email,
		}
	}

	// check if the user is already a team member by email
	exist, err := db.GetEngine(ctx).
		Where(builder.Eq{
			"team_user.org_id":  team.OrgID,
			"team_user.team_id": team.ID,
			"`user`.email":      email,
		}).
		Join("INNER", "`user`", "`user`.id = team_user.uid").
		Table("team_user").
		Exist()
	if err != nil {
		return nil, err
	}

	if exist {
		return nil, ErrUserEmailAlreadyAdded{
			Email: email,
		}
	}

	token, err := util.CryptoRandomString(25)
	if err != nil {
		return nil, err
	}

	invite := &TeamInvite{
		Token:     token,
		InviterID: doer.ID,
		OrgID:     team.OrgID,
		TeamID:    team.ID,
		Email:     email,
	}

	return invite, db.Insert(ctx, invite)
}

func RemoveInviteByID(ctx context.Context, inviteID, teamID int64) error {
	_, err := db.DeleteByBean(ctx, &TeamInvite{
		ID:     inviteID,
		TeamID: teamID,
	})
	return err
}

func GetInvitesByTeamID(ctx context.Context, teamID int64) ([]*TeamInvite, error) {
	invites := make([]*TeamInvite, 0, 10)
	return invites, db.GetEngine(ctx).
		Where("team_id=?", teamID).
		Find(&invites)
}

func GetInviteByToken(ctx context.Context, token string) (*TeamInvite, error) {
	invite := &TeamInvite{}

	has, err := db.GetEngine(ctx).Where("token=?", token).Get(invite)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrTeamInviteNotFound{Token: token}
	}
	return invite, nil
}
