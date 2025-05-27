// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// CommitStatusState holds the state of a CommitStatus
// It can be "pending", "success", "error" and "failure"
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
	// CommitStatusSkipped is for when CommitStatus is Skipped
	CommitStatusSkipped CommitStatusState = "skipped"
)

var commitStatusPriorities = map[CommitStatusState]int{
	CommitStatusError:   0,
	CommitStatusFailure: 1,
	CommitStatusWarning: 2,
	CommitStatusPending: 3,
	CommitStatusSuccess: 4,
	CommitStatusSkipped: 5,
}

func (css CommitStatusState) String() string {
	return string(css)
}

// HasHigherPriorityThan returns true if this state has higher priority than the other
// Undefined states are considered to have the highest priority like CommitStatusError(0)
func (css CommitStatusState) HasHigherPriorityThan(other CommitStatusState) bool {
	return commitStatusPriorities[css] < commitStatusPriorities[other]
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
