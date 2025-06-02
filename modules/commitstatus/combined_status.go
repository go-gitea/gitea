// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package commitstatus

// CombinedStatusState represents the combined status of a commit.
type CombinedStatusState string

const (
	// CombinedStatusPending is for when the CombinedStatus is Pending
	CombinedStatusPending CombinedStatusState = "pending"
	// CombinedStatusSuccess is for when the CombinedStatus is Success
	CombinedStatusSuccess CombinedStatusState = "success"
	// CombinedStatusFailure is for when the CombinedStatus is Failure
	CombinedStatusFailure CombinedStatusState = "failure"
)

func (cs CombinedStatusState) String() string {
	return string(cs)
}

// IsPending represents if commit status state is pending
func (cs CombinedStatusState) IsPending() bool {
	return cs == CombinedStatusPending
}

// IsSuccess represents if commit status state is success
func (cs CombinedStatusState) IsSuccess() bool {
	return cs == CombinedStatusSuccess
}

// IsFailure represents if commit status state is failure
func (cs CombinedStatusState) IsFailure() bool {
	return cs == CombinedStatusFailure
}
