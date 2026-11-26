// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"
	"strings"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"
)

type EditOptions struct {
	Summary           optional.Option[string]
	Description       optional.Option[string]
	Severity          optional.Option[string]
	CvssVector        optional.Option[string]
	CweIDs            optional.Option[[]string]
	Vulnerabilities   optional.Option[[]*advisory_model.Vulnerability]
	ContentVersion    optional.Option[int]
	CveID             optional.Option[string]
	Credits           optional.Option[[]*advisory_model.Credit]
	LabelIDs          optional.Option[[]int64]
	CollaboratorUsers optional.Option[[]string]
	CollaboratorTeams optional.Option[[]string]
	State             optional.Option[string]
	CloseReason       optional.Option[string]
	DuplicateOf       optional.Option[string]
	ShareOriginal     optional.Option[bool]
}

func EditAdvisory(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, perms advisory_model.Permissions, opts EditOptions) error {
	editsContent := opts.Summary.Has() || opts.Description.Has() || opts.Severity.Has() || opts.CvssVector.Has() || opts.CweIDs.Has() || opts.Vulnerabilities.Has()
	managesContent := opts.CveID.Has() || opts.Credits.Has() || opts.LabelIDs.Has()
	changesCollaborators := opts.CollaboratorUsers.Has() || opts.CollaboratorTeams.Has()
	changesState := opts.State.Has() || opts.CloseReason.Has() || opts.DuplicateOf.Has() || opts.ShareOriginal.Has()
	if !perms.CanManage && (managesContent || changesCollaborators || changesState) {
		return util.NewPermissionDeniedErrorf("only repository admins can change the CVE ID, credits, labels, state and collaborators")
	}
	if editsContent && !perms.CanEdit {
		return util.NewPermissionDeniedErrorf("you cannot edit this advisory")
	}

	newState, err := stateFromEdit(ctx, advisory, opts)
	if err != nil {
		return err
	}
	var collaborators collaboratorChanges
	if changesCollaborators {
		if collaborators, err = replaceCollaborators(ctx, advisory, opts.CollaboratorUsers, opts.CollaboratorTeams); err != nil {
			return err
		}
	}
	if advisory.IsReport && newState.State == advisory_model.StateDraft {
		keepOriginalReport(advisory) // Capture the report before applying edits from the same request.
	}
	contentChanged := editsContent || managesContent
	if contentChanged {
		if err := applyContentEdit(ctx, doer, advisory, perms.CanManage, opts); err != nil {
			return err
		}
	}

	oldState := advisory.State
	var access originalAccess
	effectiveCollaborators, err := db.WithTx2(ctx, func(ctx context.Context) (collaboratorChanges, error) {
		if contentChanged {
			if err := advisory_model.UpdateAdvisory(ctx, advisory); err != nil {
				return collaboratorChanges{}, err
			}
		}
		effective, err := collaborators.apply(ctx, advisory.ID)
		if err != nil {
			return effective, err
		}
		if newState.State != 0 {
			access, err = updateState(ctx, doer, advisory, newState)
		}
		return effective, err
	})
	if err != nil {
		return err
	}
	effectiveCollaborators.notify(ctx, doer, advisory)
	if oldState != advisory.State {
		notify_service.SecurityAdvisoryStateChanged(ctx, doer, advisory, oldState)
	}
	access.notify(ctx, doer)
	return nil
}

func stateFromEdit(ctx context.Context, advisory *advisory_model.Advisory, opts EditOptions) (stateOptions, error) {
	state := advisory_model.ParseState(opts.State.Value())
	if !opts.State.Has() || state == advisory.State {
		if opts.CloseReason.Has() || opts.DuplicateOf.Has() || opts.ShareOriginal.Has() {
			return stateOptions{}, util.NewInvalidArgumentErrorf("the close reason can only be set when closing the advisory")
		}
		return stateOptions{}, nil
	}
	return newStateOptions(ctx, advisory, state, opts.CloseReason.Value(), opts.DuplicateOf.Value(), opts.ShareOriginal.Value())
}

func applyContentEdit(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, canManage bool, opts EditOptions) error {
	if !canManage {
		if err := advisory.LoadRepo(ctx); err != nil {
			return err
		}
		if isBlockedFromAdvisory(ctx, advisory, doer) {
			return user_model.ErrBlockedUser
		}
	}
	content := ContentFromAdvisory(advisory)
	if strings.TrimSpace(opts.CvssVector.Value()) != "" {
		content.Severity = ""
	}
	content.Summary = opts.Summary.ValueOrDefault(content.Summary)
	content.Description = opts.Description.ValueOrDefault(content.Description)
	content.Severity = opts.Severity.ValueOrDefault(content.Severity)
	content.CvssVector = opts.CvssVector.ValueOrDefault(content.CvssVector)
	content.CveID = opts.CveID.ValueOrDefault(content.CveID)
	content.CweIDs = opts.CweIDs.ValueOrDefault(content.CweIDs)
	content.Vulnerabilities = opts.Vulnerabilities.ValueOrDefault(content.Vulnerabilities)
	content.Credits = opts.Credits.ValueOrDefault(content.Credits)
	content.LabelIDs = opts.LabelIDs.ValueOrDefault(content.LabelIDs)
	if err := applyContent(ctx, doer, advisory, content); err != nil {
		return err
	}
	advisory.ContentVersion = opts.ContentVersion.ValueOrDefault(advisory.ContentVersion)
	return nil
}
