// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// CommitStatusState holds the state of a CommitStatus
// It can be "pending", "success", "error" and "failure"
type CommitStatusState int

const (
	// CommitStatusError is for when the CommitStatus is Error
	CommitStatusError CommitStatusState = iota + 1
	// CommitStatusFailure is for when the CommitStatus is Failure
	CommitStatusFailure
	// CommitStatusPending is for when the CommitStatus is Pending
	CommitStatusPending
	// CommitStatusSuccess is for when the CommitStatus is Success
	CommitStatusSuccess
)

var commitStatusNames = map[CommitStatusState]string{
	CommitStatusError:   "error",
	CommitStatusFailure: "failure",
	CommitStatusPending: "pending",
	CommitStatusSuccess: "success",
}

// String returns the string name of the CommitStatusState
func (css CommitStatusState) String() string {
	return commitStatusNames[css]
}

func (css CommitStatusState) IsValid() bool {
	_, ok := commitStatusNames[css]
	return ok
}

// NoBetterThan returns true if this State is no better than the given State
// You should ensure the States are valid by call IsValid()
func (css CommitStatusState) NoBetterThan(css2 CommitStatusState) bool {
	return int(css) <= int(css2)
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
