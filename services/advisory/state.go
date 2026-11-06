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
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	"gitea.dev/services/audit"
	notify_service "gitea.dev/services/notify"
)

// StateOptions closing an advisory need a reason, duplicates also the original advisory
type StateOptions struct {
	State       advisory_model.State
	CloseReason advisory_model.CloseReason
	DuplicateOf *advisory_model.Advisory
}

// NewStateOptions parses the names, the original of a duplicate is looked up in the repository of the advisory
func NewStateOptions(ctx context.Context, a *advisory_model.Advisory, state, closeReason, duplicateOf string) (StateOptions, error) {
	opts := StateOptions{State: advisory_model.ParseState(state), CloseReason: advisory_model.ParseCloseReason(closeReason)}
	if closeReason != "" && opts.CloseReason == advisory_model.CloseReasonNone {
		return opts, util.NewInvalidArgumentErrorf("invalid close reason %q", closeReason)
	}
	if duplicateOf != "" {
		original, err := advisory_model.GetAdvisoryByIdentifier(ctx, a.RepoID, duplicateOf)
		if errors.Is(err, util.ErrNotExist) {
			return opts, util.NewInvalidArgumentErrorf("advisory %q does not exist", duplicateOf)
		} else if err != nil {
			return opts, err
		}
		original.Repo = a.Repo
		opts.DuplicateOf = original
	}
	return opts, validateStateOptions(a, opts)
}

func validateStateOptions(a *advisory_model.Advisory, opts StateOptions) error {
	if !a.State.CanTransitionTo(opts.State) {
		return util.NewInvalidArgumentErrorf("cannot change the state of the advisory from %s to %s", a.State, opts.State)
	}
	if opts.State != advisory_model.StateClosed {
		if opts.CloseReason != advisory_model.CloseReasonNone || opts.DuplicateOf != nil {
			return util.NewInvalidArgumentErrorf("only closed advisories have a close reason")
		}
		return nil
	}
	if opts.CloseReason == advisory_model.CloseReasonNone {
		return util.NewInvalidArgumentErrorf("a close reason is required")
	}
	if (opts.CloseReason == advisory_model.CloseReasonDuplicate) != (opts.DuplicateOf != nil) {
		return util.NewInvalidArgumentErrorf("duplicates and only duplicates need the original advisory")
	}
	if opts.DuplicateOf != nil && opts.DuplicateOf.ID == a.ID {
		return util.NewInvalidArgumentErrorf("an advisory cannot be a duplicate of itself")
	}
	if opts.DuplicateOf != nil && opts.DuplicateOf.DuplicateOfID != 0 {
		return util.NewInvalidArgumentErrorf("the original advisory is a duplicate itself")
	}
	return nil
}

// ChangeState publishes, closes, reopens or withdraws an advisory, only repository admins may call it
func ChangeState(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, opts StateOptions) error {
	change, err := updateState(ctx, doer, a, opts)
	if err != nil {
		return err
	}
	change.notify(ctx, doer, a)
	return nil
}

// stateChange is what updateState changed, for the notifications after the transaction
type stateChange struct {
	oldState         advisory_model.State
	oldDuplicateOfID int64
	duplicateOf      *advisory_model.Advisory
	revokedAccess    bool
}

func updateState(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, opts StateOptions) (*stateChange, error) {
	if err := validateStateOptions(a, opts); err != nil {
		return nil, err
	}
	change := &stateChange{oldState: a.State, oldDuplicateOfID: a.DuplicateOfID, duplicateOf: opts.DuplicateOf}
	a.State = opts.State
	now := timeutil.TimeStampNow()
	var cols []string
	switch opts.State {
	case advisory_model.StatePublished:
		a.PublisherID, a.Publisher, a.PublishedUnix = doer.ID, doer, now
		cols = []string{"publisher_id", "published_unix"}
	case advisory_model.StateWithdrawn:
		a.WithdrawnUnix = now
		cols = []string{"withdrawn_unix"}
	case advisory_model.StateClosed, advisory_model.StateDraft:
		a.ClosedUnix = util.Iif(opts.State == advisory_model.StateClosed, now, 0)
		a.CloseReason, a.DuplicateOf, a.DuplicateOfID = opts.CloseReason, opts.DuplicateOf, 0
		if opts.DuplicateOf != nil {
			a.DuplicateOfID = opts.DuplicateOf.ID
		}
		cols = []string{"closed_unix", "close_reason", "duplicate_of_id"}
		if keepOriginalReport(a, opts) {
			cols = append(cols, "report_description")
		}
	}
	return change, db.WithTx(ctx, func(ctx context.Context) (err error) {
		if err := advisory_model.UpdateAdvisoryState(ctx, a, change.oldState, cols...); err != nil {
			return err
		}
		if a.IsReport && change.oldDuplicateOfID != 0 {
			change.revokedAccess, err = advisory_model.RemoveDuplicateReporter(ctx, change.oldDuplicateOfID, a.ReporterID)
		}
		return err
	})
}

// keepOriginalReport saves the description of a report when it is accepted, a later edit replaces only the description
func keepOriginalReport(a *advisory_model.Advisory, opts StateOptions) bool {
	if opts.State != advisory_model.StateDraft || !a.IsReport {
		return false
	}
	if a.ReportDescription == "" {
		a.ReportDescription = a.Description
	}
	return true
}

// notify also lets the reporter of a duplicate follow the original advisory
func (c *stateChange) notify(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory) {
	notify_service.SecurityAdvisoryStateChanged(ctx, doer, a, c.oldState)
	if c.revokedAccess {
		auditDuplicateAccessRevoked(ctx, doer, a, c.oldDuplicateOfID)
	}
	if a.IsReport && c.duplicateOf != nil {
		// the state has been changed already, the reporter just can't follow the original
		err := grantReporterAccessToOriginal(ctx, doer, a, c.duplicateOf)
		reporterCannotCollaborate := errors.Is(err, util.ErrNotExist) || errors.Is(err, util.ErrInvalidArgument) || errors.Is(err, user_model.ErrBlockedUser)
		if err != nil && !reporterCannotCollaborate {
			log.Error("the reporter of %d cannot follow the original %d: %v", a.ID, c.duplicateOf.ID, err)
		}
	}
}

// grantReporterAccessToOriginal adds the reporter of a duplicate as read-only collaborator
func grantReporterAccessToOriginal(ctx context.Context, doer *user_model.User, duplicate, original *advisory_model.Advisory) error {
	reporterOwnsOriginal := original.IsReport && original.ReporterID == duplicate.ReporterID
	if reporterOwnsOriginal {
		return nil
	}
	reporter, err := user_model.GetUserByID(ctx, duplicate.ReporterID)
	if err != nil {
		return err
	}
	c := collaborator{user: reporter}
	if err := validateCollaborator(ctx, original, c); err != nil {
		return err
	}
	added, err := advisory_model.AddCollaborator(ctx, c.row(original.ID, true))
	if err != nil || !added {
		return err
	}
	notify_service.SecurityAdvisoryCollaboratorAdded(ctx, doer, original, reporter, nil)
	return nil
}

func auditDuplicateAccessRevoked(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, originalID int64) {
	original, has, err := db.GetByID[advisory_model.Advisory](ctx, originalID)
	if err != nil || !has {
		log.Error("auditDuplicateAccessRevoked: original %d, has %v: %v", originalID, has, err)
		return
	}
	_, reporter, err := user_model.GetPossibleUserByID(ctx, a.ReporterID)
	if err != nil {
		log.Error("auditDuplicateAccessRevoked: reporter %d: %v", a.ReporterID, err)
		return
	}
	audit.RecordAs(ctx, doer, audit_model.SecurityAdvisoryCollaboratorRemove, a.Repo, "advisory", original.Identifier, "collaborator", audit.AdvisoryCollaboratorName(reporter, nil))
}
