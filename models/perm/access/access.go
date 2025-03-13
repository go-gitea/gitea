// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package access

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"

	"xorm.io/builder"
)

// Access represents the highest access level of a user to the repository. The only access type
// that is not in this table is the real owner of a repository. In case of an organization
// repository, the members of the owners team are in this table.
type Access struct {
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"UNIQUE(s)"`
	RepoID int64 `xorm:"UNIQUE(s)"`
	Mode   perm.AccessMode
}

func init() {
	db.RegisterModel(new(Access))
}

func accessLevel(ctx context.Context, user *user_model.User, repo *repo_model.Repository) (perm.AccessMode, error) {
	mode := perm.AccessModeNone
	var userID int64
	restricted := false

	if user != nil {
		userID = user.ID
		restricted = user.IsRestricted
	}

	if !restricted && !repo.IsPrivate {
		mode = perm.AccessModeRead
	}

	if userID == 0 {
		return mode, nil
	}

	if userID == repo.OwnerID {
		return perm.AccessModeOwner, nil
	}

	a, exist, err := db.Get[Access](ctx, builder.Eq{"user_id": userID, "repo_id": repo.ID})
	if err != nil {
		return mode, err
	} else if !exist {
		return mode, nil
	}
	return a.Mode, nil
}

func maxAccessMode(modes ...perm.AccessMode) perm.AccessMode {
	maxMode := perm.AccessModeNone
	for _, mode := range modes {
		maxMode = max(maxMode, mode)
	}
	return maxMode
}

type userAccess struct {
	User *user_model.User
	Mode perm.AccessMode
}

// updateUserAccess updates an access map so that user has at least mode
func updateUserAccess(accessMap map[int64]*userAccess, user *user_model.User, mode perm.AccessMode) {
	if ua, ok := accessMap[user.ID]; ok {
		ua.Mode = maxAccessMode(ua.Mode, mode)
	} else {
		accessMap[user.ID] = &userAccess{User: user, Mode: mode}
	}
}

// FIXME: do cross-comparison so reduce deletions and additions to the minimum?
func refreshAccesses(ctx context.Context, repo *repo_model.Repository, accessMap map[int64]*userAccess) (err error) {
	minMode := perm.AccessModeRead
	if err := repo.LoadOwner(ctx); err != nil {
		return fmt.Errorf("LoadOwner: %w", err)
	}

	// If the repo isn't private and isn't owned by a organization,
	// increase the minMode to Write.
	if !repo.IsPrivate && !repo.Owner.IsOrganization() {
		minMode = perm.AccessModeWrite
	}

	newAccesses := make([]Access, 0, len(accessMap))
	for userID, ua := range accessMap {
		if ua.Mode < minMode && !ua.User.IsRestricted {
			continue
		}

		newAccesses = append(newAccesses, Access{
			UserID: userID,
			RepoID: repo.ID,
			Mode:   ua.Mode,
		})
	}

	// Delete old accesses and insert new ones for repository.
	if _, err = db.DeleteByBean(ctx, &Access{RepoID: repo.ID}); err != nil {
		return fmt.Errorf("delete old accesses: %w", err)
	}
	if len(newAccesses) == 0 {
		return nil
	}

	if err = db.Insert(ctx, newAccesses); err != nil {
		return fmt.Errorf("insert new accesses: %w", err)
	}
	return nil
}

// refreshCollaboratorAccesses retrieves repository collaborations with their access modes.
func refreshCollaboratorAccesses(ctx context.Context, repoID int64, accessMap map[int64]*userAccess) error {
	collaborators, _, err := repo_model.GetCollaborators(ctx, &repo_model.FindCollaborationOptions{RepoID: repoID})
	if err != nil {
		return fmt.Errorf("GetCollaborators: %w", err)
	}
	for _, c := range collaborators {
		if c.User.IsGhost() {
			continue
		}
		updateUserAccess(accessMap, c.User, c.Collaboration.Mode)
	}
	return nil
}

// RecalculateTeamAccesses recalculates new accesses for teams of an organization
// except the team whose ID is given. It is used to assign a team ID when
// remove repository from that team.
func RecalculateTeamAccesses(ctx context.Context, repo *repo_model.Repository, ignTeamID int64) (err error) {
	accessMap := make(map[int64]*userAccess, 20)

	if err = repo.LoadOwner(ctx); err != nil {
		return err
	} else if !repo.Owner.IsOrganization() {
		return fmt.Errorf("owner is not an organization: %d", repo.OwnerID)
	}

	if err = refreshCollaboratorAccesses(ctx, repo.ID, accessMap); err != nil {
		return fmt.Errorf("refreshCollaboratorAccesses: %w", err)
	}

	teams, err := organization.FindOrgTeams(ctx, repo.Owner.ID)
	if err != nil {
		return err
	}

	for _, t := range teams {
		if t.ID == ignTeamID {
			continue
		}

		// Owner team gets owner access, and skip for teams that do not
		// have relations with repository.
		if t.IsOwnerTeam() {
			t.AccessMode = perm.AccessModeOwner
		} else if !organization.HasTeamRepo(ctx, t.OrgID, t.ID, repo.ID) {
			continue
		}

		if err = t.LoadMembers(ctx); err != nil {
			return fmt.Errorf("getMembers '%d': %w", t.ID, err)
		}
		for _, m := range t.Members {
			updateUserAccess(accessMap, m, t.AccessMode)
		}
	}

	return refreshAccesses(ctx, repo, accessMap)
}

// RecalculateUserAccess recalculates new access for a single user
// Usable if we know access only affected one user
func RecalculateUserAccess(ctx context.Context, repo *repo_model.Repository, uid int64) (err error) {
	minMode := perm.AccessModeRead
	if !repo.IsPrivate {
		minMode = perm.AccessModeWrite
	}

	accessMode := perm.AccessModeNone
	e := db.GetEngine(ctx)
	collaborator, err := repo_model.GetCollaboration(ctx, repo.ID, uid)
	if err != nil {
		return err
	} else if collaborator != nil {
		accessMode = collaborator.Mode
	}

	if err = repo.LoadOwner(ctx); err != nil {
		return err
	} else if repo.Owner.IsOrganization() {
		var teams []organization.Team
		if err := e.Join("INNER", "team_repo", "team_repo.team_id = team.id").
			Join("INNER", "team_user", "team_user.team_id = team.id").
			Where("team.org_id = ?", repo.OwnerID).
			And("team_repo.repo_id=?", repo.ID).
			And("team_user.uid=?", uid).
			Find(&teams); err != nil {
			return err
		}

		for _, t := range teams {
			if t.IsOwnerTeam() {
				t.AccessMode = perm.AccessModeOwner
			}

			accessMode = maxAccessMode(accessMode, t.AccessMode)
		}
	}

	// Delete old user accesses and insert new one for repository.
	if _, err = e.Delete(&Access{RepoID: repo.ID, UserID: uid}); err != nil {
		return fmt.Errorf("delete old user accesses: %w", err)
	} else if accessMode >= minMode {
		if err = db.Insert(ctx, &Access{RepoID: repo.ID, UserID: uid, Mode: accessMode}); err != nil {
			return fmt.Errorf("insert new user accesses: %w", err)
		}
	}
	return nil
}

// RecalculateAccesses recalculates all accesses for repository.
func RecalculateAccesses(ctx context.Context, repo *repo_model.Repository) error {
	if repo.Owner.IsOrganization() {
		return RecalculateTeamAccesses(ctx, repo, 0)
	}

	accessMap := make(map[int64]*userAccess, 20)
	if err := refreshCollaboratorAccesses(ctx, repo.ID, accessMap); err != nil {
		return fmt.Errorf("refreshCollaboratorAccesses: %w", err)
	}
	return refreshAccesses(ctx, repo, accessMap)
}
