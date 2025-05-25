// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package commitstatus

// CombinedStatus represents the combined status of a commit.
type CombinedStatus string

const (
	// CombinedStatusPending is for when the CombinedStatus is Pending
	CombinedStatusPending CombinedStatus = "pending"
	// CombinedStatusSuccess is for when the CombinedStatus is Success
	CombinedStatusSuccess CombinedStatus = "success"
	// CombinedStatusFailure is for when the CombinedStatus is Failure
	CombinedStatusFailure CombinedStatus = "failure"
)

func (cs CombinedStatus) String() string {
	return string(cs)
}

// IsPending represents if commit status state is pending
func (cs CombinedStatus) IsPending() bool {
	return cs == CombinedStatusPending
}

// IsSuccess represents if commit status state is success
func (cs CombinedStatus) IsSuccess() bool {
	return cs == CombinedStatusSuccess
}

// IsFailure represents if commit status state is failure
func (cs CombinedStatus) IsFailure() bool {
	return cs == CombinedStatusFailure
}
