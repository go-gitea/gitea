// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package commitstatus

// CommitStatusState holds the state of a CommitStatus
// swagger:enum CommitStatusState
type CommitStatusState string //nolint:revive // export stutter

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
	// CommitStatusRunningWithFailure is for only aggregated commit status
	CommitStatusRunningWithFailure CommitStatusState = "running_with_failure"
)

func (css CommitStatusState) String() string {
	return string(css)
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

// IsSkipped represents if commit status state is skipped
func (css CommitStatusState) IsSkipped() bool {
	return css == CommitStatusSkipped
}

// IsRunningWithFailure represents if commit status state is running with failure
func (css CommitStatusState) IsRunningWithFailure() bool {
	return css == CommitStatusRunningWithFailure
}

type CommitStatusStates []CommitStatusState //nolint:revive // export stutter

// According to https://docs.github.com/en/rest/commits/statuses?apiVersion=2022-11-28#get-the-combined-status-for-a-specific-reference
// > Additionally, a combined state is returned. The state is one of:
// > failure if any of the contexts report as error or failure and no contexts are pending
// > pending if there are no statuses or a context is pending with no failure
// > running_with_failure if there are contexts that are pending and at least one context is failure
// > success if the latest status for all contexts is success
func (css CommitStatusStates) Combine() CommitStatusState {
	successCnt := 0
	hasRunning, hasFailure := false, false
	for _, state := range css {
		switch {
		case state.IsError() || state.IsFailure():
			hasFailure = true
		case state.IsPending():
			hasRunning = true
		case state.IsSuccess() || state.IsWarning() || state.IsSkipped():
			successCnt++
		}
	}
	if successCnt > 0 && successCnt == len(css) {
		return CommitStatusSuccess
	}
	if hasFailure {
		if hasRunning {
			return CommitStatusRunningWithFailure
		}
		return CommitStatusFailure
	}
	return CommitStatusPending
}
