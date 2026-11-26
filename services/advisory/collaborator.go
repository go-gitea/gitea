// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"
	"errors"
	"slices"

	advisory_model "gitea.dev/models/advisory"
	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	access_model "gitea.dev/models/perm/access"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"
	"gitea.dev/services/audit"
	notify_service "gitea.dev/services/notify"
)

type collaborator struct {
	user *user_model.User
	team *organization.Team
}

func (c collaborator) row(advisoryID int64) *advisory_model.Collaborator {
	row := &advisory_model.Collaborator{AdvisoryID: advisoryID}
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

func currentCollaborators(advisory *advisory_model.Advisory) []collaborator {
	entries := make([]collaborator, 0, len(advisory.CollaboratorUsers)+len(advisory.CollaboratorTeams))
	for _, user := range advisory.CollaboratorUsers {
		entries = append(entries, collaborator{user: user})
	}
	for _, team := range advisory.CollaboratorTeams {
		entries = append(entries, collaborator{team: team})
	}
	return entries
}

func lookupCollaborator(ctx context.Context, advisory *advisory_model.Advisory, name string, isTeam bool) (collaborator, error) {
	if err := advisory.LoadRepo(ctx); err != nil {
		return collaborator{}, err
	}
	if isTeam {
		team, err := organization.GetTeam(ctx, advisory.Repo.OwnerID, name)
		return collaborator{team: team}, err
	}
	user, err := user_model.GetUserByName(ctx, name)
	return collaborator{user: user}, err
}

func validateCollaborator(ctx context.Context, advisory *advisory_model.Advisory, entry collaborator) error {
	if err := advisory.Repo.LoadOwner(ctx); err != nil {
		return err
	}
	if team := entry.team; team != nil {
		if advisory.Repo.IsPrivate && !(organization.HasTeamRepo(ctx, team.OrgID, team.ID, advisory.RepoID) && team.UnitEnabled(ctx, unit.TypeSecurityAdvisories)) {
			return util.NewInvalidArgumentErrorf("team %s cannot access the advisories of the repository", team.Name)
		}
		return nil
	}

	user := entry.user
	if !user.IsIndividual() {
		return util.NewInvalidArgumentErrorf("only users can collaborate on advisories")
	}
	if isBlockedFromAdvisory(ctx, advisory, user) || user_model.IsUserBlockedBy(ctx, advisory.Repo.Owner, user.ID) {
		return user_model.ErrBlockedUser
	}
	perm, err := access_model.GetIndividualUserRepoPermission(ctx, advisory.Repo, user)
	if err != nil {
		return err
	}
	if !perm.CanRead(unit.TypeSecurityAdvisories) {
		return util.NewInvalidArgumentErrorf("user %s cannot access the advisories of the repository", user.Name)
	}
	return nil
}

type collaboratorChanges struct {
	added   []collaborator
	removed []collaborator
}

func (changes collaboratorChanges) apply(ctx context.Context, advisoryID int64) (collaboratorChanges, error) {
	var effective collaboratorChanges
	for _, entry := range changes.removed {
		removed, err := advisory_model.RemoveCollaborator(ctx, entry.row(advisoryID))
		if err != nil {
			return effective, err
		}
		if removed {
			effective.removed = append(effective.removed, entry)
		}
	}
	for _, entry := range changes.added {
		added, err := advisory_model.AddCollaborator(ctx, entry.row(advisoryID))
		if err != nil {
			return effective, err
		}
		if added {
			effective.added = append(effective.added, entry)
		}
	}
	return effective, nil
}

func (changes collaboratorChanges) notify(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory) {
	for _, entry := range changes.removed {
		audit.RecordAs(ctx, doer, audit_model.SecurityAdvisoryCollaboratorRemove, advisory.Repo, "advisory", advisory.Identifier, "collaborator", audit.AdvisoryCollaboratorName(entry.user, entry.team))
	}
	for _, entry := range changes.added {
		notify_service.SecurityAdvisoryCollaboratorAdded(ctx, doer, advisory, entry.user, entry.team)
	}
}

func changeCollaborators(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, changes collaboratorChanges) error {
	effective, err := db.WithTx2(ctx, func(ctx context.Context) (collaboratorChanges, error) {
		return changes.apply(ctx, advisory.ID)
	})
	if err != nil {
		return err
	}
	effective.notify(ctx, doer, advisory)
	return nil
}

func AddCollaborator(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, name string, isTeam bool) error {
	entry, err := lookupCollaborator(ctx, advisory, name, isTeam)
	if err != nil {
		return err
	}
	if err := validateCollaborator(ctx, advisory, entry); err != nil {
		return err
	}
	return changeCollaborators(ctx, doer, advisory, collaboratorChanges{added: []collaborator{entry}})
}

func RemoveCollaborator(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, userID, teamID int64) error {
	if err := advisory.LoadCollaborators(ctx); err != nil {
		return err
	}
	var changes collaboratorChanges
	for _, entry := range currentCollaborators(advisory) {
		if entry.user != nil && entry.user.ID == userID || entry.team != nil && entry.team.ID == teamID {
			changes.removed = append(changes.removed, entry)
		}
	}
	return changeCollaborators(ctx, doer, advisory, changes)
}

func replaceCollaborators(ctx context.Context, advisory *advisory_model.Advisory, optUserNames, optTeamNames optional.Option[[]string]) (collaboratorChanges, error) {
	if err := advisory.LoadCollaborators(ctx); err != nil {
		return collaboratorChanges{}, err
	}
	current := currentCollaborators(advisory)
	userNames, teamNames := optUserNames.Value(), optTeamNames.Value()
	var wanted []collaborator
	for _, existing := range current {
		if existing.user != nil && !optUserNames.Has() || existing.team != nil && !optTeamNames.Has() {
			wanted = append(wanted, existing)
		}
	}
	if len(wanted)+len(userNames)+len(teamNames) > maxListLength {
		return collaboratorChanges{}, util.NewInvalidArgumentErrorf("at most %d collaborators are allowed", maxListLength)
	}
	for i, name := range slices.Concat(userNames, teamNames) {
		isTeam := i >= len(userNames)
		entry, err := lookupCollaborator(ctx, advisory, name, isTeam)
		if errors.Is(err, util.ErrNotExist) {
			return collaboratorChanges{}, util.NewInvalidArgumentErrorf("%s %q does not exist", util.Iif(isTeam, "team", "user"), name)
		} else if err != nil {
			return collaboratorChanges{}, err
		}
		wanted = append(wanted, entry)
	}

	var changes collaboratorChanges
	for _, entry := range current {
		if !slices.ContainsFunc(wanted, entry.is) {
			changes.removed = append(changes.removed, entry)
		}
	}
	for _, entry := range wanted {
		if slices.ContainsFunc(current, entry.is) {
			continue // Existing collaborators may have lost the permissions required to add them.
		}
		if err := validateCollaborator(ctx, advisory, entry); err != nil {
			return collaboratorChanges{}, err
		}
		changes.added = append(changes.added, entry)
	}
	return changes, nil
}
