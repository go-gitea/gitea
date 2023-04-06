// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// CommitStatusState holds the state of a CommitStatus
// It can be "pending", "success", "error", "failure", and "warning"
type CommitStatusState string

const (
	// CommitStatusPending is for when the CommitStatus is Pending
	CommitStatusPending CommitStatusState = "pending"
	// CommitStatusSuccess is for when the CommitStatus is Success
	CommitStatusSuccess CommitStatusState = "success"
	// CommitStatusError is for when the CommitStatus is Error
	CommitStatusError CommitStatusState = "error"
	// CommitStatusFailure is for when the CommitStatus is Failure
	CommitStatusFailure CommitStatusState = "failure"
	// CommitStatusWarning is for when the CommitStatus is Warning
	CommitStatusWarning CommitStatusState = "warning"
	// CommitStatusRunning is for when the CommitStatus is Running
	CommitStatusRunning CommitStatusState = "running"

	// status used in check runs, should not be used as commit status:

	// CommitStatusNeutral is for when the CommitStatus is neutral
	CommitStatusNeutral CommitStatusState = "neutral"
	// CommitStatusSkipped is for when the CommitStatus is neutral
	CommitStatusSkipped CommitStatusState = "skipped"
	// CommitStatusTimedOut is for when the CommitStatus is timed_out
	CommitStatusTimedOut CommitStatusState = "timed_out"
)

// NoBetterThan returns true if this State is no better than the given State
func (css CommitStatusState) NoBetterThan(css2 CommitStatusState) bool {
	switch css {
	case CommitStatusTimedOut:
		return true
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

// IsSkiped represents if commit status state is skipped
func (css CommitStatusState) IsSkipped() bool {
	return css == CommitStatusSkipped
}

// IsNeutral represents if commit status state is neutral
func (css CommitStatusState) IsNeutral() bool {
	return css == CommitStatusNeutral
}

// IsTimedOut represents if commit status state is timed_out
func (css CommitStatusState) IsTimedOut() bool {
	return css == CommitStatusTimedOut
}
