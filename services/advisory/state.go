// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"
	"errors"

	advisory_model "gitea.dev/models/advisory"
	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	"gitea.dev/services/audit"
	notify_service "gitea.dev/services/notify"

	"xorm.io/builder"
)

type stateOptions struct {
	State         advisory_model.State
	CloseReason   advisory_model.CloseReason
	DuplicateOf   *advisory_model.Advisory
	ShareOriginal bool
}

func newStateOptions(ctx context.Context, advisory *advisory_model.Advisory, state advisory_model.State, closeReason, duplicateOf string, shareOriginal bool) (stateOptions, error) {
	opts := stateOptions{State: state, CloseReason: advisory_model.ParseCloseReason(closeReason), ShareOriginal: shareOriginal}
	if closeReason != "" && opts.CloseReason == advisory_model.CloseReasonNone {
		return opts, util.NewInvalidArgumentErrorf("invalid close reason %q", closeReason)
	}
	if duplicateOf != "" {
		original, err := advisory_model.GetAdvisoryByIdentifier(ctx, advisory.RepoID, duplicateOf)
		if errors.Is(err, util.ErrNotExist) {
			return opts, util.NewInvalidArgumentErrorf("advisory %q does not exist", duplicateOf)
		} else if err != nil {
			return opts, err
		}
		original.Repo = advisory.Repo
		opts.DuplicateOf = original
	}
	return opts, validateStateOptions(advisory, opts)
}

func validateStateOptions(advisory *advisory_model.Advisory, opts stateOptions) error {
	if !advisory.State.CanTransitionTo(opts.State) {
		return util.NewInvalidArgumentErrorf("cannot change the state of the advisory from %s to %s", advisory.State, opts.State)
	}
	if opts.State == advisory_model.StateTriage && advisory.ReopenState() != advisory_model.StateTriage {
		return util.NewInvalidArgumentErrorf("only reports that were never accepted can return to triage")
	}
	if opts.State != advisory_model.StateClosed {
		if opts.CloseReason != advisory_model.CloseReasonNone || opts.DuplicateOf != nil || opts.ShareOriginal {
			return util.NewInvalidArgumentErrorf("only closed advisories have a close reason")
		}
		return nil
	}
	if (opts.CloseReason == advisory_model.CloseReasonDuplicate) != (opts.DuplicateOf != nil) {
		return util.NewInvalidArgumentErrorf("duplicates and only duplicates need the original advisory")
	}
	if opts.ShareOriginal && (opts.DuplicateOf == nil || !advisory.IsReport) {
		return util.NewInvalidArgumentErrorf("the original can only be shared with the reporter of a duplicate report")
	}
	if original := opts.DuplicateOf; original != nil {
		switch {
		case original.ID == advisory.ID:
			return util.NewInvalidArgumentErrorf("an advisory cannot be a duplicate of itself")
		case original.DuplicateOfID != 0:
			return util.NewInvalidArgumentErrorf("the original advisory is a duplicate itself")
		}
	}
	return nil
}

// ChangeState requires the caller to check repository admin permission.
func ChangeState(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, state, closeReason, duplicateOf string, shareOriginal bool) error {
	opts, err := newStateOptions(ctx, advisory, advisory_model.ParseState(state), closeReason, duplicateOf, shareOriginal)
	if err != nil {
		return err
	}
	oldState := advisory.State
	access, err := updateState(ctx, doer, advisory, opts)
	if err != nil {
		return err
	}
	notify_service.SecurityAdvisoryStateChanged(ctx, doer, advisory, oldState)
	access.notify(ctx, doer)
	return nil
}

func updateState(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, opts stateOptions) (originalAccess, error) {
	previous := &advisory_model.Advisory{ID: advisory.ID, Repo: advisory.Repo, ReporterID: advisory.ReporterID, DuplicateOfID: advisory.DuplicateOfID, ShareOriginal: advisory.ShareOriginal}
	oldState := advisory.State
	advisory.State = opts.State
	now := timeutil.TimeStampNow()
	var cols []string
	switch opts.State {
	case advisory_model.StatePublished:
		advisory.PublisherID, advisory.Publisher, advisory.PublishedUnix = doer.ID, doer, now
		cols = []string{"publisher_id", "published_unix"}
	case advisory_model.StateWithdrawn:
		advisory.WithdrawnUnix = now
		cols = []string{"withdrawn_unix"}
	case advisory_model.StateClosed, advisory_model.StateDraft, advisory_model.StateTriage:
		advisory.ClosedUnix = util.Iif(opts.State == advisory_model.StateClosed, now, 0)
		advisory.CloseReason, advisory.DuplicateOf, advisory.DuplicateOfID, advisory.ShareOriginal = opts.CloseReason, opts.DuplicateOf, 0, opts.ShareOriginal
		if opts.DuplicateOf != nil {
			advisory.DuplicateOfID = opts.DuplicateOf.ID
		}
		cols = []string{"closed_unix", "close_reason", "duplicate_of_id", "share_original"}
		if advisory.IsReport && opts.State == advisory_model.StateDraft {
			keepOriginalReport(advisory)
			cols = append(cols, "report_description")
		}
	}
	return db.WithTx2(ctx, func(ctx context.Context) (originalAccess, error) {
		if opts.DuplicateOf != nil { // two admins closing a pair as duplicates of each other at the same instant is not guarded
			if has, err := db.Exist[advisory_model.Advisory](ctx, builder.Eq{"duplicate_of_id": advisory.ID}); err != nil {
				return originalAccess{}, err
			} else if has {
				return originalAccess{}, util.NewInvalidArgumentErrorf("advisories with duplicates cannot be duplicates themselves")
			}
		}
		if err := advisory_model.UpdateAdvisoryState(ctx, advisory, oldState, cols...); err != nil {
			return originalAccess{}, err
		}
		if opts.ShareOriginal {
			return shareOriginal(ctx, advisory, opts.DuplicateOf)
		}
		return revokeSharedOriginal(ctx, previous)
	})
}

type originalAccess struct {
	original *advisory_model.Advisory
	reporter *user_model.User
	granted  bool
}

func (access originalAccess) notify(ctx context.Context, doer *user_model.User) {
	switch {
	case access.original == nil:
	case access.granted:
		notify_service.SecurityAdvisoryCollaboratorAdded(ctx, doer, access.original, access.reporter, nil)
	default:
		audit.RecordAs(ctx, doer, audit_model.SecurityAdvisoryCollaboratorRemove, access.original.Repo, "advisory", access.original.Identifier, "collaborator", audit.AdvisoryCollaboratorName(access.reporter, nil))
	}
}

func shareOriginal(ctx context.Context, duplicate, original *advisory_model.Advisory) (originalAccess, error) {
	if original.IsReport && original.ReporterID == duplicate.ReporterID {
		return originalAccess{}, nil
	}
	if err := original.LoadRepo(ctx); err != nil {
		return originalAccess{}, err
	}
	reporter, err := user_model.GetUserByID(ctx, duplicate.ReporterID)
	if err != nil {
		return originalAccess{}, err
	}
	follower := collaborator{user: reporter}
	if err := validateCollaborator(ctx, original, follower); err != nil {
		return originalAccess{}, err
	}
	row := follower.row(original.ID)
	row.ReadOnly = true
	added, err := advisory_model.AddCollaborator(ctx, row)
	if err != nil || !added {
		return originalAccess{}, err
	}
	return originalAccess{original: original, reporter: reporter, granted: true}, nil
}

func revokeSharedOriginal(ctx context.Context, duplicate *advisory_model.Advisory) (originalAccess, error) {
	revoked, err := advisory_model.RevokeSharedOriginal(ctx, duplicate)
	if err != nil || !revoked {
		return originalAccess{}, err
	}
	original, _, err := db.GetByID[advisory_model.Advisory](ctx, duplicate.DuplicateOfID)
	if err != nil || original == nil {
		return originalAccess{}, err
	}
	original.Repo = duplicate.Repo
	_, reporter, err := user_model.GetPossibleUserByID(ctx, duplicate.ReporterID)
	return originalAccess{original: original, reporter: reporter}, err
}

func keepOriginalReport(advisory *advisory_model.Advisory) {
	if advisory.ReportDescription == "" {
		advisory.ReportDescription = advisory.Description
	}
}
