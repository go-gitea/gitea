// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package commitstatus

// CombinedStatusState represents the combined status of a commit.
type CombinedStatusState string

const (
	// CombinedStatusStatePending is for when the CombinedStatus is Pending
	CombinedStatusStatePending CombinedStatusState = "pending"
	// CombinedStatusStateSuccess is for when the CombinedStatus is Success
	CombinedStatusStateSuccess CombinedStatusState = "success"
	// CombinedStatusStateFailure is for when the CombinedStatus is Failure
	CombinedStatusStateFailure CombinedStatusState = "failure"
)

func (cs CombinedStatusState) String() string {
	return string(cs)
}

// IsPending represents if commit status state is pending
func (cs CombinedStatusState) IsPending() bool {
	return cs == CombinedStatusStatePending
}

// IsSuccess represents if commit status state is success
func (cs CombinedStatusState) IsSuccess() bool {
	return cs == CombinedStatusStateSuccess
}

// IsFailure represents if commit status state is failure
func (cs CombinedStatusState) IsFailure() bool {
	return cs == CombinedStatusStateFailure
}
