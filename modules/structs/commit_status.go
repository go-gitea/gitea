// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// CommitStatusState holds the state of a Status
// It can be "pending", "success", "error", "failure", and "warning"
type CommitStatusState string

const (
	// CommitStatusPending is for when the Status is Pending
	CommitStatusPending CommitStatusState = "pending"
	// CommitStatusSuccess is for when the Status is Success
	CommitStatusSuccess CommitStatusState = "success"
	// CommitStatusError is for when the Status is Error
	CommitStatusError CommitStatusState = "error"
	// CommitStatusFailure is for when the Status is Failure
	CommitStatusFailure CommitStatusState = "failure"
	// CommitStatusWarning is for when the Status is Warning
	CommitStatusWarning CommitStatusState = "warning"
)

// NoBetterThan returns true if this State is no better than the given State
func (css CommitStatusState) NoBetterThan(css2 CommitStatusState) bool {
	switch css {
	case CommitStatusError:
		return true
	case CommitStatusFailure:
		return css2 != CommitStatusError
	case CommitStatusWarning:
		return css2 != CommitStatusError && css2 != CommitStatusFailure
	case CommitStatusPending:
		return css2 != CommitStatusError && css2 != CommitStatusFailure && css2 != CommitStatusWarning
	default:
		return css2 != CommitStatusError && css2 != CommitStatusFailure && css2 != CommitStatusWarning && css2 != CommitStatusPending
	}
}

// IsPending represents if commit status state is pending
func (css CommitStatusState) IsPending() bool {
	return css == CommitStatusPending
}

// IsSuccess represents if commit status state is success
func (css CommitStatusState) IsSuccess() bool {
	return css == CommitStatusSuccess
}

// IsError represents if commit status state is error
func (css CommitStatusState) IsError() bool {
	return css == CommitStatusError
}

// IsFailure represents if commit status state is failure
func (css CommitStatusState) IsFailure() bool {
	return css == CommitStatusFailure
}

// IsWarning represents if commit status state is warning
func (css CommitStatusState) IsWarning() bool {
	return css == CommitStatusWarning
}
