// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"
)

// EditOptions are the changes of an advisory, unset fields are kept
type EditOptions struct {
	Summary         optional.Option[string]
	Description     optional.Option[string]
	Severity        optional.Option[string]
	CvssV3Vector    optional.Option[string]
	CvssV4Vector    optional.Option[string]
	CweIDs          optional.Option[[]string]
	Vulnerabilities optional.Option[[]*advisory_model.Vulnerability]
	ContentVersion  optional.Option[int] // the edit is rejected if the advisory has been changed since, by default since it was loaded

	// only repository admins can change the rest
	CveID             optional.Option[string]
	Credits           optional.Option[[]*advisory_model.Credit]
	LabelIDs          optional.Option[[]int64]
	CollaboratorUsers optional.Option[[]string] // user names
	CollaboratorTeams optional.Option[[]string] // team names of the repository owner
	State             optional.Option[string]
	CloseReason       optional.Option[string]
	DuplicateOf       optional.Option[string] // identifier of the original advisory
}

func (opts EditOptions) editsContent() bool {
	return opts.Summary.Has() || opts.Description.Has() || opts.Severity.Has() || opts.CvssV3Vector.Has() ||
		opts.CvssV4Vector.Has() || opts.CweIDs.Has() || opts.Vulnerabilities.Has()
}

func (opts EditOptions) managesContent() bool {
	return opts.CveID.Has() || opts.Credits.Has() || opts.LabelIDs.Has()
}

func (opts EditOptions) changesCollaborators() bool {
	return opts.CollaboratorUsers.Has() || opts.CollaboratorTeams.Has()
}

func (opts EditOptions) changesState() bool {
	return opts.State.Has() || opts.CloseReason.Has() || opts.DuplicateOf.Has()
}

func checkEditPermission(perms advisory_model.Permissions, opts EditOptions) error {
	if (opts.managesContent() || opts.changesCollaborators() || opts.changesState()) && !perms.CanManage {
		return util.NewPermissionDeniedErrorf("only repository admins can change the CVE ID, credits, labels, state and collaborators")
	}
	if opts.editsContent() && !perms.CanEdit {
		return util.NewPermissionDeniedErrorf("you cannot edit this advisory")
	}
	return nil
}

// EditAdvisory changes an advisory with loaded attributes in one transaction, after validating all changes
func EditAdvisory(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, perms advisory_model.Permissions, opts EditOptions) error {
	if err := checkEditPermission(perms, opts); err != nil {
		return err
	}
	newState, err := stateFromEdit(ctx, a, opts)
	if err != nil {
		return err
	}
	var collaborators collaboratorChanges
	if opts.changesCollaborators() {
		if collaborators, err = replaceCollaborators(ctx, a, opts.CollaboratorUsers, opts.CollaboratorTeams); err != nil {
			return err
		}
	}
	contentChanged := opts.editsContent() || opts.managesContent()
	if contentChanged {
		if err := applyContentEdit(ctx, doer, a, perms, opts); err != nil {
			return err
		}
	}

	var effectiveCollaborators collaboratorChanges
	var stateChanged *stateChange
	err = db.WithTx(ctx, func(ctx context.Context) (err error) {
		if contentChanged {
			if err := advisory_model.UpdateAdvisory(ctx, a, "summary", "description", "cve_id", "severity",
				"cvss_v3_vector", "cvss_v3_score_tenths", "cvss_v4_vector", "cvss_v4_score_tenths", "cwe_ids"); err != nil {
				return err
			}
		}
		if effectiveCollaborators, err = collaborators.apply(ctx, a.ID); err != nil {
			return err
		}
		if newState != nil {
			stateChanged, err = updateState(ctx, doer, a, *newState)
		}
		return err
	})
	if err != nil {
		return err
	}
	effectiveCollaborators.notify(ctx, doer, a)
	if stateChanged != nil {
		return stateChanged.notify(ctx, doer, a)
	}
	return nil
}

// stateFromEdit returns nil if the state doesn't change
func stateFromEdit(ctx context.Context, a *advisory_model.Advisory, opts EditOptions) (*StateOptions, error) {
	if !opts.State.Has() || advisory_model.ParseState(opts.State.Value()) == a.State {
		if opts.CloseReason.Has() || opts.DuplicateOf.Has() {
			return nil, util.NewInvalidArgumentErrorf("the close reason can only be set when closing the advisory")
		}
		return nil, nil //nolint:nilnil // the state doesn't change
	}
	stateOpts, err := NewStateOptions(ctx, a, opts.State.Value(), opts.CloseReason.Value(), opts.DuplicateOf.Value())
	if err != nil {
		return nil, err
	}
	return &stateOpts, nil
}

// applyContentEdit changes the content of the advisory in memory, if it is valid
func applyContentEdit(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, perms advisory_model.Permissions, opts EditOptions) error {
	if !perms.CanManage {
		if err := a.LoadRepo(ctx); err != nil {
			return err
		}
		if isBlockedFromAdvisory(ctx, a, doer) {
			return user_model.ErrBlockedUser
		}
	}
	content := ContentFromAdvisory(a)
	if opts.CvssV3Vector.Value() != "" || opts.CvssV4Vector.Value() != "" {
		content.Severity = "" // a new vector replaces the manual severity
	}
	content.Summary = opts.Summary.ValueOrDefault(content.Summary)
	content.Description = opts.Description.ValueOrDefault(content.Description)
	content.Severity = opts.Severity.ValueOrDefault(content.Severity)
	content.CvssV3Vector = opts.CvssV3Vector.ValueOrDefault(content.CvssV3Vector)
	content.CvssV4Vector = opts.CvssV4Vector.ValueOrDefault(content.CvssV4Vector)
	content.CweIDs = opts.CweIDs.ValueOrDefault(content.CweIDs)
	content.Vulnerabilities = opts.Vulnerabilities.ValueOrDefault(content.Vulnerabilities)
	content.CveID = opts.CveID.ValueOrDefault(content.CveID)
	content.Credits = opts.Credits.ValueOrDefault(content.Credits)
	content.LabelIDs = opts.LabelIDs.ValueOrDefault(content.LabelIDs)
	if err := applyContent(ctx, doer, a, content); err != nil {
		return err
	}
	a.ContentVersion = opts.ContentVersion.ValueOrDefault(a.ContentVersion)
	return nil
}
