// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"
	"errors"
	"slices"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	access_model "gitea.dev/models/perm/access"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"
)

// collaborator is either a user or a team
type collaborator struct {
	user *user_model.User
	team *organization.Team
}

func (c collaborator) row(advisoryID int64, readOnly bool) *advisory_model.Collaborator {
	row := &advisory_model.Collaborator{AdvisoryID: advisoryID, ReadOnly: readOnly}
	if c.user != nil {
		row.UserID = c.user.ID
	} else {
		row.TeamID = c.team.ID
	}
	return row
}

func (c collaborator) is(other collaborator) bool {
	if c.user != nil {
		return other.user != nil && other.user.ID == c.user.ID
	}
	return other.team != nil && other.team.ID == c.team.ID
}

func currentCollaborators(a *advisory_model.Advisory) []collaborator {
	res := make([]collaborator, 0, len(a.CollaboratorUsers)+len(a.CollaboratorTeams))
	for _, u := range a.CollaboratorUsers {
		res = append(res, collaborator{user: u})
	}
	for _, t := range a.CollaboratorTeams {
		res = append(res, collaborator{team: t})
	}
	return res
}

// lookupCollaborator finds a user or a team of the repository owner by name
func lookupCollaborator(ctx context.Context, a *advisory_model.Advisory, name string, isTeam bool) (collaborator, error) {
	if err := a.LoadRepo(ctx); err != nil {
		return collaborator{}, err
	}
	if isTeam {
		t, err := organization.GetTeam(ctx, a.Repo.OwnerID, name)
		return collaborator{team: t}, err
	}
	u, err := user_model.GetUserByName(ctx, name)
	return collaborator{user: u}, err
}

// validateCollaborator checks that a user or team can read the advisories unit and a team belongs to the repository owner
func validateCollaborator(ctx context.Context, a *advisory_model.Advisory, c collaborator) error {
	if err := a.LoadRepo(ctx); err != nil {
		return err
	}
	if err := a.Repo.LoadOwner(ctx); err != nil {
		return err
	}
	if c.team != nil {
		t := c.team
		if t.OrgID != a.Repo.OwnerID {
			return util.NewInvalidArgumentErrorf("team %s does not belong to the repository owner", t.Name)
		}
		if a.Repo.IsPrivate && !(organization.HasTeamRepo(ctx, t.OrgID, t.ID, a.RepoID) && t.UnitEnabled(ctx, unit.TypeSecurityAdvisories)) {
			return util.NewInvalidArgumentErrorf("team %s cannot access the advisories of the repository", t.Name)
		}
		return nil
	}

	u := c.user
	if !u.IsIndividual() {
		return util.NewInvalidArgumentErrorf("only users can collaborate on advisories")
	}
	if isBlockedFromAdvisory(ctx, a, u) || user_model.IsUserBlockedBy(ctx, a.Repo.Owner, u.ID) {
		return user_model.ErrBlockedUser
	}
	perm, err := access_model.GetIndividualUserRepoPermission(ctx, a.Repo, u)
	if err != nil {
		return err
	}
	if !perm.CanRead(unit.TypeSecurityAdvisories) {
		return util.NewInvalidArgumentErrorf("user %s cannot access the advisories of the repository", u.Name)
	}
	return nil
}

// collaboratorChanges are the users and teams which get write access, also read-only collaborators, and which lose access
type collaboratorChanges struct {
	added   []collaborator
	removed []collaborator
}

// apply returns the effective changes, which are notified after the transaction
func (changes collaboratorChanges) apply(ctx context.Context, advisoryID int64) (effective collaboratorChanges, err error) {
	for _, c := range changes.removed {
		removed, err := advisory_model.RemoveCollaborator(ctx, c.row(advisoryID, false))
		if err != nil {
			return effective, err
		}
		if removed {
			effective.removed = append(effective.removed, c)
		}
	}
	for _, c := range changes.added {
		added, err := addWritableCollaborator(ctx, advisoryID, c)
		if err != nil {
			return effective, err
		}
		if added {
			effective.added = append(effective.added, c)
		}
	}
	return effective, nil
}

func addWritableCollaborator(ctx context.Context, advisoryID int64, c collaborator) (bool, error) {
	added, err := advisory_model.AddCollaborator(ctx, c.row(advisoryID, false))
	if err != nil || added {
		return added, err
	}
	return advisory_model.SetCollaboratorWritable(ctx, c.row(advisoryID, false))
}

func (changes collaboratorChanges) notify(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory) {
	for _, c := range changes.removed {
		notify_service.SecurityAdvisoryCollaboratorRemoved(ctx, doer, a, c.user, c.team)
	}
	for _, c := range changes.added {
		notify_service.SecurityAdvisoryCollaboratorAdded(ctx, doer, a, c.user, c.team)
	}
}

func changeCollaborators(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, changes collaboratorChanges) error {
	var effective collaboratorChanges
	err := db.WithTx(ctx, func(ctx context.Context) (err error) {
		effective, err = changes.apply(ctx, a.ID)
		return err
	})
	if err != nil {
		return err
	}
	effective.notify(ctx, doer, a)
	return nil
}

// AddCollaborator grants a user or a team of the repository owner write access to an advisory, by name
func AddCollaborator(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, name string, isTeam bool) error {
	c, err := lookupCollaborator(ctx, a, name, isTeam)
	if err != nil {
		return err
	}
	if err := validateCollaborator(ctx, a, c); err != nil {
		return err
	}
	return changeCollaborators(ctx, doer, a, collaboratorChanges{added: []collaborator{c}})
}

// RemoveCollaborator revokes the access of a user or a team, only current collaborators are removed,
// which also limits the teams to the repository owner
func RemoveCollaborator(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, userID, teamID int64) error {
	if err := a.LoadCollaborators(ctx); err != nil {
		return err
	}
	var changes collaboratorChanges
	for _, c := range currentCollaborators(a) {
		if c.user != nil && c.user.ID == userID || c.team != nil && c.team.ID == teamID {
			changes.removed = append(changes.removed, c)
		}
	}
	return changeCollaborators(ctx, doer, a, changes)
}

// replaceCollaborators validates the new users and teams, the current ones are kept even if they couldn't be added anymore.
// Unset lists are kept, unknown names are invalid like other invalid fields of an edit.
func replaceCollaborators(ctx context.Context, a *advisory_model.Advisory, userNames, teamNames optional.Option[[]string]) (collaboratorChanges, error) {
	if err := a.LoadCollaborators(ctx); err != nil {
		return collaboratorChanges{}, err
	}
	current := currentCollaborators(a)
	var wanted []collaborator
	if !userNames.Has() {
		wanted = slices.DeleteFunc(slices.Clone(current), func(c collaborator) bool { return c.user == nil })
	}
	if !teamNames.Has() {
		wanted = append(wanted, slices.DeleteFunc(slices.Clone(current), func(c collaborator) bool { return c.team == nil })...)
	}
	if len(wanted)+len(userNames.Value())+len(teamNames.Value()) > maxListLength {
		return collaboratorChanges{}, util.NewInvalidArgumentErrorf("at most %d collaborators are allowed", maxListLength)
	}
	for _, name := range userNames.Value() {
		c, err := lookupNewCollaborator(ctx, a, name, false)
		if err != nil {
			return collaboratorChanges{}, err
		}
		wanted = append(wanted, c)
	}
	for _, name := range teamNames.Value() {
		c, err := lookupNewCollaborator(ctx, a, name, true)
		if err != nil {
			return collaboratorChanges{}, err
		}
		wanted = append(wanted, c)
	}

	var changes collaboratorChanges
	for _, c := range current {
		if !slices.ContainsFunc(wanted, c.is) {
			changes.removed = append(changes.removed, c)
		}
	}
	for _, c := range wanted {
		isCurrent := slices.ContainsFunc(current, c.is)
		if !isCurrent {
			if err := validateCollaborator(ctx, a, c); err != nil {
				return collaboratorChanges{}, err
			}
		}
		changes.added = append(changes.added, c) // also grants write access to current read-only collaborators
	}
	return changes, nil
}

func lookupNewCollaborator(ctx context.Context, a *advisory_model.Advisory, name string, isTeam bool) (collaborator, error) {
	c, err := lookupCollaborator(ctx, a, name, isTeam)
	if errors.Is(err, util.ErrNotExist) {
		return c, util.NewInvalidArgumentErrorf("%s %q does not exist", util.Iif(isTeam, "team", "user"), name)
	}
	return c, err
}
