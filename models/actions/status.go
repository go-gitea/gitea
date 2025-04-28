// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"code.gitea.io/gitea/modules/translation"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
)

// Status represents the status of ActionRun, ActionRunJob, ActionTask, or ActionTaskStep
type Status int

const (
	StatusUnknown   Status = iota // 0, consistent with runnerv1.Result_RESULT_UNSPECIFIED
	StatusSuccess                 // 1, consistent with runnerv1.Result_RESULT_SUCCESS
	StatusFailure                 // 2, consistent with runnerv1.Result_RESULT_FAILURE
	StatusCancelled               // 3, consistent with runnerv1.Result_RESULT_CANCELLED
	StatusSkipped                 // 4, consistent with runnerv1.Result_RESULT_SKIPPED
	StatusWaiting                 // 5, isn't a runnerv1.Result
	StatusRunning                 // 6, isn't a runnerv1.Result
	StatusBlocked                 // 7, isn't a runnerv1.Result
)

var statusNames = map[Status]string{
	StatusUnknown:   "unknown",
	StatusWaiting:   "waiting",
	StatusRunning:   "running",
	StatusSuccess:   "success",
	StatusFailure:   "failure",
	StatusCancelled: "cancelled",
	StatusSkipped:   "skipped",
	StatusBlocked:   "blocked",
}

// String returns the string name of the Status
func (s Status) String() string {
	return statusNames[s]
}

// LocaleString returns the locale string name of the Status
func (s Status) LocaleString(lang translation.Locale) string {
	return lang.TrString("actions.status." + s.String())
}

// IsDone returns whether the Status is final
func (s Status) IsDone() bool {
	return s.In(StatusSuccess, StatusFailure, StatusCancelled, StatusSkipped)
}

// HasRun returns whether the Status is a result of running
func (s Status) HasRun() bool {
	return s.In(StatusSuccess, StatusFailure)
}

func (s Status) IsUnknown() bool {
	return s == StatusUnknown
}

func (s Status) IsSuccess() bool {
	return s == StatusSuccess
}

func (s Status) IsFailure() bool {
	return s == StatusFailure
}

func (s Status) IsCancelled() bool {
	return s == StatusCancelled
}

func (s Status) IsSkipped() bool {
	return s == StatusSkipped
}

func (s Status) IsWaiting() bool {
	return s == StatusWaiting
}

func (s Status) IsRunning() bool {
	return s == StatusRunning
}

func (s Status) IsBlocked() bool {
	return s == StatusBlocked
}

// In returns whether s is one of the given statuses
func (s Status) In(statuses ...Status) bool {
	for _, v := range statuses {
		if s == v {
			return true
		}
	}
	return false
}

func (s Status) AsResult() runnerv1.Result {
	if s.IsDone() {
		return runnerv1.Result(s)
	}
	return runnerv1.Result_RESULT_UNSPECIFIED
}
