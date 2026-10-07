// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"
	"slices"

	"gitea.dev/models/db"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"

	"xorm.io/builder"
)

// Viewer is a user who can read the advisories unit of a repository, only its admins and the participants see non-public advisories
type Viewer struct {
	Doer        *user_model.User
	IsRepoAdmin bool
	PublicOnly  bool // the request is authenticated by a token restricted to public resources
}

func NewViewer(doer *user_model.User, perm access_model.Permission, publicOnly bool) Viewer {
	return Viewer{Doer: doer, IsRepoAdmin: perm.IsAdmin(), PublicOnly: publicOnly}
}

// canSeeNonPublic excludes anonymous users, bots like the Actions user and public-only tokens
func (v Viewer) canSeeNonPublic() bool {
	return v.Doer != nil && v.Doer.IsIndividual() && !v.PublicOnly
}

// CanManage reports whether the viewer can create advisories and change their state, collaborators, credits and CVE ID
func (v Viewer) CanManage() bool {
	return v.canSeeNonPublic() && v.IsRepoAdmin
}

func collaboratorCond(userID int64) builder.Cond {
	return builder.Or(
		builder.Eq{"user_id": userID},
		builder.In("team_id", builder.Select("team_id").From("team_user").Where(builder.Eq{"uid": userID})),
	)
}

func (v Viewer) visibilityCond() builder.Cond {
	public := builder.In("state", StatePublished, StateWithdrawn)
	if !v.canSeeNonPublic() {
		return public
	}
	if v.IsRepoAdmin {
		return builder.NewCond()
	}
	return builder.Or(
		public,
		builder.Eq{"is_report": true, "reporter_id": v.Doer.ID},
		builder.In("id", builder.Select("advisory_id").From("security_advisory_collaborator").Where(collaboratorCond(v.Doer.ID))),
	)
}

// DiscussionIDs returns the IDs of the advisories whose private discussion the viewer can see
func (v Viewer) DiscussionIDs(ctx context.Context, list List) (container.Set[int64], error) {
	ids := make(container.Set[int64])
	if !v.canSeeNonPublic() {
		return ids, nil
	}
	var others []int64
	for _, a := range list {
		if v.IsRepoAdmin || a.IsReport && a.ReporterID == v.Doer.ID {
			ids.Add(a.ID)
		} else {
			others = append(others, a.ID)
		}
	}
	if len(others) == 0 {
		return ids, nil
	}
	var collaborating []int64
	err := db.GetEngine(ctx).Table("security_advisory_collaborator").Cols("advisory_id").
		In("advisory_id", others).And(collaboratorCond(v.Doer.ID)).Find(&collaborating)
	ids.AddMultiple(collaborating...)
	return ids, err
}

type Permissions struct {
	CanView          bool
	CanSeeDiscussion bool
	CanEdit          bool // participants can edit until the advisory is published, except read-only collaborators
}

func (v Viewer) Permissions(ctx context.Context, a *Advisory) (p Permissions, err error) {
	switch {
	case !v.canSeeNonPublic():
	case v.IsRepoAdmin:
		p = Permissions{CanSeeDiscussion: true, CanEdit: true}
	case a.IsReport && a.ReporterID == v.Doer.ID:
		p = Permissions{CanSeeDiscussion: true, CanEdit: !a.IsPublic()}
	default:
		var readOnly []bool
		err = db.GetEngine(ctx).Table("security_advisory_collaborator").Cols("read_only").
			Where("advisory_id = ?", a.ID).And(collaboratorCond(v.Doer.ID)).Find(&readOnly)
		p.CanSeeDiscussion = len(readOnly) > 0
		p.CanEdit = !a.IsPublic() && slices.Contains(readOnly, false)
	}
	p.CanView = a.IsPublic() || p.CanSeeDiscussion
	return p, err
}

// GetAdvisory returns ErrAdvisoryNotExist also if the viewer cannot see the advisory
func (v Viewer) GetAdvisory(ctx context.Context, repo *repo_model.Repository, identifier string) (*Advisory, Permissions, error) {
	a, err := GetAdvisoryByIdentifier(ctx, repo.ID, identifier)
	if err != nil {
		return nil, Permissions{}, err
	}
	a.Repo = repo
	perms, err := v.Permissions(ctx, a)
	if err != nil {
		return nil, Permissions{}, err
	} else if !perms.CanView {
		return nil, Permissions{}, ErrAdvisoryNotExist{RepoID: repo.ID, Identifier: identifier}
	}
	return a, perms, nil
}

// HideUnviewableOriginals keeps only the close reason of the duplicates whose original the viewer cannot see
func (v Viewer) HideUnviewableOriginals(ctx context.Context, list List) error {
	originals := container.FilterSlice(list, func(a *Advisory) (*Advisory, bool) { return a.DuplicateOf, a.DuplicateOf != nil })
	visible, err := v.DiscussionIDs(ctx, originals)
	if err != nil {
		return err
	}
	for _, a := range list {
		if a.DuplicateOf != nil && !a.DuplicateOf.IsPublic() && !visible.Contains(a.DuplicateOf.ID) {
			a.DuplicateOf = nil
		}
	}
	return nil
}

// ParticipantIDs returns the reporter of a private report and the collaborating users including the members of the teams
func ParticipantIDs(ctx context.Context, a *Advisory) (container.Set[int64], error) {
	ids := make(container.Set[int64])
	if a.IsReport {
		ids.Add(a.ReporterID)
	}
	var userIDs []int64
	if err := db.GetEngine(ctx).Table("security_advisory_collaborator").Cols("user_id").
		Where("advisory_id = ? AND user_id > 0", a.ID).Find(&userIDs); err != nil {
		return nil, err
	}
	var memberIDs []int64
	if err := db.GetEngine(ctx).Table("team_user").Cols("uid").Where(builder.In("team_id",
		builder.Select("team_id").From("security_advisory_collaborator").Where(builder.Eq{"advisory_id": a.ID}.And(builder.Gt{"team_id": 0})),
	)).Find(&memberIDs); err != nil {
		return nil, err
	}
	ids.AddMultiple(userIDs...)
	ids.AddMultiple(memberIDs...)
	return ids, nil
}

// UserCanSeeDiscussion checks the current permission of a user, who might have lost access since being notified before
func UserCanSeeDiscussion(ctx context.Context, user *user_model.User, a *Advisory) (bool, error) {
	if err := a.LoadRepo(ctx); err != nil {
		return false, err
	}
	perm, err := access_model.GetIndividualUserRepoPermission(ctx, a.Repo, user)
	if err != nil {
		return false, err
	}
	if !perm.CanRead(unit.TypeSecurityAdvisories) {
		return false, nil
	}
	p, err := NewViewer(user, perm, false).Permissions(ctx, a)
	return p.CanSeeDiscussion, err
}
