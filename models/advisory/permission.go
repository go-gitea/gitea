// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"
	"fmt"

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

// canSeeNonPublic excludes anonymous users, bots like the Actions user and public-only tokens
func (v Viewer) canSeeNonPublic() bool {
	return v.Doer != nil && v.Doer.IsIndividual() && !v.PublicOnly
}

// CanManage reports whether the viewer can create advisories and change their state, collaborators, credits and CVE ID
func (v Viewer) CanManage() bool {
	return v.canSeeNonPublic() && v.IsRepoAdmin
}

// isOwnReport reports whether the viewer is the reporter of a private report, who participates in it like a collaborator
func (v Viewer) isOwnReport(a *Advisory) bool {
	return a.IsReport && a.ReporterID == v.Doer.ID
}

// collaboratorCond matches the collaborator rows of the user and the user's teams
func collaboratorCond(userID int64) builder.Cond {
	return builder.Or(
		builder.Eq{"user_id": userID},
		builder.In("team_id", builder.Select("team_id").From("team_user").Where(builder.Eq{"uid": userID})),
	)
}

// collaboratorAccess maps the advisories the user collaborates on to whether the user can edit them
func collaboratorAccess(ctx context.Context, userID int64, advisoryIDs []int64) (map[int64]bool, error) {
	writable := make(map[int64]bool)
	if len(advisoryIDs) == 0 {
		return writable, nil
	}
	var rows []*Collaborator
	if err := db.GetEngine(ctx).In("advisory_id", advisoryIDs).And(collaboratorCond(userID)).Find(&rows); err != nil {
		return nil, err
	}
	for _, c := range rows {
		writable[c.AdvisoryID] = writable[c.AdvisoryID] || !c.ReadOnly // any writable row of the user or a team grants write access
	}
	return writable, nil
}

// visibilityCond is the SQL form of Permissions.CanView
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
		if v.IsRepoAdmin || v.isOwnReport(a) {
			ids.Add(a.ID)
		} else {
			others = append(others, a.ID)
		}
	}
	collaborating, err := collaboratorAccess(ctx, v.Doer.ID, others)
	if err != nil {
		return nil, err
	}
	for id := range collaborating {
		ids.Add(id)
	}
	return ids, nil
}

type Permissions struct {
	CanView          bool
	CanSeeDiscussion bool
	CanEdit          bool // participants can edit until the advisory is published, except read-only collaborators
	CanManage        bool // see Viewer.CanManage
}

func (v Viewer) Permissions(ctx context.Context, a *Advisory) (Permissions, error) {
	if !v.canSeeNonPublic() {
		return Permissions{CanView: a.IsPublic()}, nil
	}
	if v.IsRepoAdmin {
		return Permissions{CanView: true, CanSeeDiscussion: true, CanEdit: true, CanManage: true}, nil
	}
	if v.isOwnReport(a) {
		return Permissions{CanView: true, CanSeeDiscussion: true, CanEdit: !a.IsPublic()}, nil
	}
	collaborating, err := collaboratorAccess(ctx, v.Doer.ID, []int64{a.ID})
	if err != nil {
		return Permissions{}, err
	}
	writable, isCollaborator := collaborating[a.ID]
	return Permissions{
		CanView:          a.IsPublic() || isCollaborator,
		CanSeeDiscussion: isCollaborator,
		CanEdit:          writable && !a.IsPublic(),
	}, nil
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

// LoadForDisplay loads the attributes and, where the viewer can see the discussion, the collaborators.
// It returns the IDs of the latter, whose private details the viewer can see too.
func (v Viewer) LoadForDisplay(ctx context.Context, list List) (container.Set[int64], error) {
	if err := list.LoadAttributes(ctx); err != nil {
		return nil, fmt.Errorf("LoadForDisplay: LoadAttributes: %w", err)
	}
	if err := v.hideUnviewableOriginals(ctx, list); err != nil {
		return nil, fmt.Errorf("LoadForDisplay: hideUnviewableOriginals: %w", err)
	}
	discussionIDs, err := v.DiscussionIDs(ctx, list)
	if err != nil {
		return nil, fmt.Errorf("LoadForDisplay: DiscussionIDs: %w", err)
	}
	withDiscussion := container.FilterSlice(list, func(a *Advisory) (*Advisory, bool) { return a, discussionIDs.Contains(a.ID) })
	if err := List(withDiscussion).LoadCollaborators(ctx); err != nil {
		return nil, fmt.Errorf("LoadForDisplay: LoadCollaborators: %w", err)
	}
	return discussionIDs, nil
}

// hideUnviewableOriginals keeps only the close reason of the duplicates whose original the viewer cannot see
func (v Viewer) hideUnviewableOriginals(ctx context.Context, list List) error {
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
	p, err := Viewer{Doer: user, IsRepoAdmin: perm.IsAdmin()}.Permissions(ctx, a)
	return p.CanSeeDiscussion, err
}
