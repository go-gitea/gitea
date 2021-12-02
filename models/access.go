// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	user_model "code.gitea.io/gitea/models/user"
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

func accessLevel(e db.Engine, user *user_model.User, repo *Repository) (perm.AccessMode, error) {
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

	a := &Access{UserID: userID, RepoID: repo.ID}
	if has, err := e.Get(a); !has || err != nil {
		return mode, err
	}
	return a.Mode, nil
}

func maxAccessMode(modes ...perm.AccessMode) perm.AccessMode {
	max := perm.AccessModeNone
	for _, mode := range modes {
		if mode > max {
			max = mode
		}
	}
	return max
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
func (repo *Repository) refreshAccesses(e db.Engine, accessMap map[int64]*userAccess) (err error) {
	minMode := perm.AccessModeRead
	if !repo.IsPrivate {
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
	if _, err = e.Delete(&Access{RepoID: repo.ID}); err != nil {
		return fmt.Errorf("delete old accesses: %v", err)
	}
	if len(newAccesses) == 0 {
		return nil
	}

	if _, err = e.Insert(newAccesses); err != nil {
		return fmt.Errorf("insert new accesses: %v", err)
	}
	return nil
}

// refreshCollaboratorAccesses retrieves repository collaborations with their access modes.
func (repo *Repository) refreshCollaboratorAccesses(e db.Engine, accessMap map[int64]*userAccess) error {
	collaborators, err := repo.getCollaborators(e, db.ListOptions{})
	if err != nil {
		return fmt.Errorf("getCollaborations: %v", err)
	}
	for _, c := range collaborators {
		if c.User.IsGhost() {
			continue
		}
		updateUserAccess(accessMap, c.User, c.Collaboration.Mode)
	}
	return nil
}

// recalculateTeamAccesses recalculates new accesses for teams of an organization
// except the team whose ID is given. It is used to assign a team ID when
// remove repository from that team.
func (repo *Repository) recalculateTeamAccesses(e db.Engine, ignTeamID int64) (err error) {
	accessMap := make(map[int64]*userAccess, 20)

	if err = repo.getOwner(e); err != nil {
		return err
	} else if !repo.Owner.IsOrganization() {
		return fmt.Errorf("owner is not an organization: %d", repo.OwnerID)
	}

	if err = repo.refreshCollaboratorAccesses(e, accessMap); err != nil {
		return fmt.Errorf("refreshCollaboratorAccesses: %v", err)
	}

	teams, err := OrgFromUser(repo.Owner).loadTeams(e)
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
			t.Authorize = perm.AccessModeOwner
		} else if !t.hasRepository(e, repo.ID) {
			continue
		}

		if err = t.getMembers(e); err != nil {
			return fmt.Errorf("getMembers '%d': %v", t.ID, err)
		}
		for _, m := range t.Members {
			updateUserAccess(accessMap, m, t.Authorize)
		}
	}

	return repo.refreshAccesses(e, accessMap)
}

// recalculateUserAccess recalculates new access for a single user
// Usable if we know access only affected one user
func (repo *Repository) recalculateUserAccess(e db.Engine, uid int64) (err error) {
	minMode := perm.AccessModeRead
	if !repo.IsPrivate {
		minMode = perm.AccessModeWrite
	}

	accessMode := perm.AccessModeNone
	collaborator, err := repo.getCollaboration(e, uid)
	if err != nil {
		return err
	} else if collaborator != nil {
		accessMode = collaborator.Mode
	}

	if err = repo.getOwner(e); err != nil {
		return err
	} else if repo.Owner.IsOrganization() {
		var teams []Team
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
				t.Authorize = perm.AccessModeOwner
			}

			accessMode = maxAccessMode(accessMode, t.Authorize)
		}
	}

	// Delete old user accesses and insert new one for repository.
	if _, err = e.Delete(&Access{RepoID: repo.ID, UserID: uid}); err != nil {
		return fmt.Errorf("delete old user accesses: %v", err)
	} else if accessMode >= minMode {
		if _, err = e.Insert(&Access{RepoID: repo.ID, UserID: uid, Mode: accessMode}); err != nil {
			return fmt.Errorf("insert new user accesses: %v", err)
		}
	}
	return nil
}

func (repo *Repository) recalculateAccesses(e db.Engine) error {
	if repo.Owner.IsOrganization() {
		return repo.recalculateTeamAccesses(e, 0)
	}

	accessMap := make(map[int64]*userAccess, 20)
	if err := repo.refreshCollaboratorAccesses(e, accessMap); err != nil {
		return fmt.Errorf("refreshCollaboratorAccesses: %v", err)
	}
	return repo.refreshAccesses(e, accessMap)
}

// RecalculateAccesses recalculates all accesses for repository.
func (repo *Repository) RecalculateAccesses() error {
	return repo.recalculateAccesses(db.GetEngine(db.DefaultContext))
}
