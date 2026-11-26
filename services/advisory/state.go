// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"
	"errors"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"

	"xorm.io/builder"
)

type stateOptions struct {
	State       advisory_model.State
	CloseReason advisory_model.CloseReason
	DuplicateOf *advisory_model.Advisory
}

func newStateOptions(ctx context.Context, advisory *advisory_model.Advisory, state advisory_model.State, closeReason, duplicateOf string) (stateOptions, error) {
	opts := stateOptions{State: state, CloseReason: advisory_model.ParseCloseReason(closeReason)}
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
	if opts.State != advisory_model.StateClosed {
		if opts.CloseReason != advisory_model.CloseReasonNone || opts.DuplicateOf != nil {
			return util.NewInvalidArgumentErrorf("only closed advisories have a close reason")
		}
		return nil
	}
	if (opts.CloseReason == advisory_model.CloseReasonDuplicate) != (opts.DuplicateOf != nil) {
		return util.NewInvalidArgumentErrorf("duplicates and only duplicates need the original advisory")
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
func ChangeState(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, state, closeReason, duplicateOf string) error {
	opts, err := newStateOptions(ctx, advisory, advisory_model.ParseState(state), closeReason, duplicateOf)
	if err != nil {
		return err
	}
	oldState := advisory.State
	if err := updateState(ctx, doer, advisory, opts); err != nil {
		return err
	}
	notify_service.SecurityAdvisoryStateChanged(ctx, doer, advisory, oldState)
	return nil
}

func updateState(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, opts stateOptions) error {
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
	case advisory_model.StateClosed, advisory_model.StateDraft:
		advisory.ClosedUnix = util.Iif(opts.State == advisory_model.StateClosed, now, 0)
		advisory.CloseReason, advisory.DuplicateOf, advisory.DuplicateOfID = opts.CloseReason, opts.DuplicateOf, 0
		if opts.DuplicateOf != nil {
			advisory.DuplicateOfID = opts.DuplicateOf.ID
		}
		cols = []string{"closed_unix", "close_reason", "duplicate_of_id"}
		if advisory.IsReport && opts.State == advisory_model.StateDraft {
			keepOriginalReport(advisory)
			cols = append(cols, "report_description")
		}
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		if opts.DuplicateOf != nil {
			if has, err := db.Exist[advisory_model.Advisory](ctx, builder.Eq{"duplicate_of_id": advisory.ID}); err != nil {
				return err
			} else if has {
				return util.NewInvalidArgumentErrorf("advisories with duplicates cannot be duplicates themselves")
			}
		}
		return advisory_model.UpdateAdvisoryState(ctx, advisory, oldState, cols...)
	})
}

func keepOriginalReport(advisory *advisory_model.Advisory) {
	if advisory.ReportDescription == "" {
		advisory.ReportDescription = advisory.Description
	}
}
