// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"
	"fmt"
	"slices"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"

	"xorm.io/builder"
)

type Viewer struct {
	Doer                 *user_model.User
	IsRepoAdmin          bool
	IsRepoWriter         bool
	IsSecurityTeamMember bool
	PublicOnly           bool
}

func NewViewer(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, perm access_model.Permission, publicOnly bool) (Viewer, error) {
	viewer := Viewer{Doer: doer, IsRepoAdmin: perm.IsAdmin(), IsRepoWriter: perm.CanWrite(unit.TypeCode), PublicOnly: publicOnly}
	if !viewer.canSeeNonPublic() || viewer.IsRepoAdmin {
		return viewer, nil
	}
	var err error
	viewer.IsSecurityTeamMember, err = organization.IsSecurityTeamMember(ctx, doer.ID, repo.ID)
	return viewer, err
}

func (v Viewer) seesAllNonPublic() bool {
	return v.IsRepoAdmin || v.IsSecurityTeamMember
}

func (v Viewer) canSeeNonPublic() bool {
	return v.Doer != nil && v.Doer.IsIndividual() && !v.PublicOnly
}

func (v Viewer) CanManage() bool {
	return v.canSeeNonPublic() && v.IsRepoAdmin
}

func (v Viewer) isOwnReport(advisory *Advisory) bool {
	return advisory.IsReport && advisory.ReporterID == v.Doer.ID
}

func collaboratorCond(userID int64) builder.Cond {
	return builder.Or(
		builder.Eq{"user_id": userID},
		builder.In("team_id", builder.Select("team_id").From("team_user").Where(builder.Eq{"uid": userID})),
	)
}

// visibilityCond is the SQL form of Permissions.CanView
func (v Viewer) visibilityCond() builder.Cond {
	public := builder.In("state", StatePublished, StateWithdrawn)
	if !v.canSeeNonPublic() {
		return public
	}
	if v.seesAllNonPublic() {
		return builder.NewCond()
	}
	return builder.Or(public, v.discussionCond())
}

// discussionCond is the SQL form of Permissions.CanSeeDiscussion
func (v Viewer) discussionCond() builder.Cond {
	if !v.canSeeNonPublic() {
		return builder.Expr("1=0")
	}
	if v.seesAllNonPublic() {
		return builder.NewCond()
	}
	return builder.Or(
		builder.Eq{"is_report": true, "reporter_id": v.Doer.ID},
		builder.In("id", advisoryIDsFrom("security_advisory_collaborator", collaboratorCond(v.Doer.ID))),
	)
}

// DiscussionIDs returns the IDs of the advisories whose private details the viewer can see
func (v Viewer) DiscussionIDs(ctx context.Context, list List) (container.Set[int64], error) {
	ids := make(container.Set[int64])
	if !v.canSeeNonPublic() {
		return ids, nil
	}
	var others []int64
	for _, advisory := range list {
		if v.seesAllNonPublic() || v.isOwnReport(advisory) {
			ids.Add(advisory.ID)
		} else {
			others = append(others, advisory.ID)
		}
	}
	if len(others) > 0 {
		collaborating, err := db.FindIDs(ctx, "security_advisory_collaborator", "advisory_id",
			builder.In("advisory_id", others).And(collaboratorCond(v.Doer.ID)))
		if err != nil {
			return nil, err
		}
		ids.AddMultiple(collaborating...)
	}
	return ids, nil
}

type Permissions struct {
	CanView            bool
	CanSeeDiscussion   bool
	CanSeeConfidential bool
	CanComment         bool
	CanEdit            bool
	CanManage          bool
}

func (v Viewer) Permissions(ctx context.Context, advisory *Advisory) (Permissions, error) {
	if !v.canSeeNonPublic() {
		return Permissions{CanView: advisory.IsPublic()}, nil
	}
	var rows []*Collaborator
	if !v.seesAllNonPublic() {
		if err := db.GetEngine(ctx).Where(builder.Eq{"advisory_id": advisory.ID}.And(collaboratorCond(v.Doer.ID))).Cols("read_only").Find(&rows); err != nil {
			return Permissions{}, err
		}
	}
	collaborates := v.seesAllNonPublic() || slices.ContainsFunc(rows, func(row *Collaborator) bool { return !row.ReadOnly })
	canSeeDiscussion := collaborates || len(rows) > 0 || v.isOwnReport(advisory)
	return Permissions{
		CanView:            advisory.IsPublic() || canSeeDiscussion,
		CanSeeDiscussion:   canSeeDiscussion,
		CanSeeConfidential: v.seesAllNonPublic() || collaborates && v.IsRepoWriter, // like GitHub's confidential comments, only for people with write access
		CanComment:         collaborates || v.isOwnReport(advisory),
		CanEdit:            v.IsRepoAdmin || ((collaborates || v.isOwnReport(advisory)) && !advisory.IsPublic() && advisory.State != StateClosed),
		CanManage:          v.IsRepoAdmin,
	}, nil
}

// GetAdvisory returns ErrAdvisoryNotExist also if the viewer cannot see the advisory
func (v Viewer) GetAdvisory(ctx context.Context, repo *repo_model.Repository, identifier string) (*Advisory, Permissions, error) {
	advisory, err := GetAdvisoryByIdentifier(ctx, repo.ID, identifier)
	if err != nil {
		return nil, Permissions{}, err
	}
	advisory.Repo = repo
	perms, err := v.Permissions(ctx, advisory)
	if err != nil {
		return nil, Permissions{}, err
	} else if !perms.CanView {
		return nil, Permissions{}, ErrAdvisoryNotExist{RepoID: repo.ID, Identifier: identifier}
	}
	return advisory, perms, nil
}

// LoadForDisplay returns the IDs of advisories whose private details the viewer can see.
func (v Viewer) LoadForDisplay(ctx context.Context, list List) (container.Set[int64], error) {
	if err := list.LoadAttributes(ctx); err != nil {
		return nil, fmt.Errorf("LoadForDisplay: LoadAttributes: %w", err)
	}
	if err := v.limitUnviewableOriginals(ctx, list); err != nil {
		return nil, fmt.Errorf("LoadForDisplay: limitUnviewableOriginals: %w", err)
	}
	discussionIDs, err := v.DiscussionIDs(ctx, list)
	if err != nil {
		return nil, fmt.Errorf("LoadForDisplay: discussionIDs: %w", err)
	}
	withDiscussion := container.FilterSlice(list, func(advisory *Advisory) (*Advisory, bool) { return advisory, discussionIDs.Contains(advisory.ID) })
	if err := List(withDiscussion).LoadCollaborators(ctx); err != nil {
		return nil, fmt.Errorf("LoadForDisplay: LoadCollaborators: %w", err)
	}
	return discussionIDs, nil
}

func (v Viewer) limitUnviewableOriginals(ctx context.Context, list List) error {
	originals := container.FilterSlice(list, func(advisory *Advisory) (*Advisory, bool) { return advisory.DuplicateOf, advisory.DuplicateOf != nil })
	visible, err := v.DiscussionIDs(ctx, originals)
	if err != nil {
		return err
	}
	for _, advisory := range list {
		if original := advisory.DuplicateOf; original != nil && !original.IsPublic() && !visible.Contains(original.ID) {
			advisory.DuplicateOf = &Advisory{
				ID: original.ID, RepoID: original.RepoID, Repo: original.Repo, Identifier: original.Identifier, Index: original.Index,
				State: original.State, Severity: original.Severity, CreatedUnix: original.CreatedUnix, IsMetadataOnly: true,
			}
		}
	}
	return nil
}

func ParticipantIDs(ctx context.Context, advisory *Advisory) (container.Set[int64], error) {
	collaborators := builder.Eq{"advisory_id": advisory.ID}
	userIDs, err := db.FindIDs(ctx, "security_advisory_collaborator", "user_id", collaborators.And(builder.Gt{"user_id": 0}))
	if err != nil {
		return nil, err
	}
	teams := builder.Select("team_id").From("security_advisory_collaborator").Where(collaborators.And(builder.Gt{"team_id": 0}))
	memberIDs, err := db.FindIDs(ctx, "team_user", "uid", builder.In("team_id", teams))
	if err != nil {
		return nil, err
	}
	ids := container.SetOf(userIDs...)
	ids.AddMultiple(memberIDs...)
	if advisory.IsReport {
		ids.Add(advisory.ReporterID)
	}
	return ids, nil
}

// UserPermissions checks the current permissions of a user, who might have lost access since being notified before
func UserPermissions(ctx context.Context, user *user_model.User, advisory *Advisory) (Permissions, error) {
	if err := advisory.LoadRepo(ctx); err != nil {
		return Permissions{}, err
	}
	perm, err := access_model.GetIndividualUserRepoPermission(ctx, advisory.Repo, user)
	if err != nil {
		return Permissions{}, err
	}
	if !perm.CanRead(unit.TypeSecurityAdvisories) {
		return Permissions{}, nil
	}
	viewer, err := NewViewer(ctx, user, advisory.Repo, perm, false)
	if err != nil {
		return Permissions{}, err
	}
	return viewer.Permissions(ctx, advisory)
}
