// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"slices"

	runnerv1 "gitea.dev/actions-proto-go/runner/v1"
	"gitea.dev/modules/translation"
)

// Status represents the status of ActionRun, ActionRunJob, ActionTask, or ActionTaskStep
type Status int

const (
	StatusUnknown    Status = iota // 0, consistent with runnerv1.Result_RESULT_UNSPECIFIED
	StatusSuccess                  // 1, consistent with runnerv1.Result_RESULT_SUCCESS
	StatusFailure                  // 2, consistent with runnerv1.Result_RESULT_FAILURE
	StatusCancelled                // 3, consistent with runnerv1.Result_RESULT_CANCELLED
	StatusSkipped                  // 4, consistent with runnerv1.Result_RESULT_SKIPPED
	StatusWaiting                  // 5, isn't a runnerv1.Result
	StatusRunning                  // 6, isn't a runnerv1.Result
	StatusBlocked                  // 7, isn't a runnerv1.Result
	StatusCancelling               // 8, isn't a runnerv1.Result
)

var statusNames = map[Status]string{
	StatusUnknown:    "unknown",
	StatusWaiting:    "waiting",
	StatusRunning:    "running",
	StatusSuccess:    "success",
	StatusFailure:    "failure",
	StatusCancelled:  "cancelled",
	StatusCancelling: "cancelling",
	StatusSkipped:    "skipped",
	StatusBlocked:    "blocked",
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

func (s Status) IsCancelling() bool {
	return s == StatusCancelling
}

// In returns whether s is one of the given statuses
func (s Status) In(statuses ...Status) bool {
	return slices.Contains(statuses, s)
}

func (s Status) AsResult() runnerv1.Result {
	switch s {
	case StatusSuccess:
		return runnerv1.Result_RESULT_SUCCESS
	case StatusFailure:
		return runnerv1.Result_RESULT_FAILURE
	case StatusCancelled, StatusCancelling:
		return runnerv1.Result_RESULT_CANCELLED
	case StatusSkipped:
		return runnerv1.Result_RESULT_SKIPPED
	default:
		return runnerv1.Result_RESULT_UNSPECIFIED
	}
}

func StatusFromResult(r runnerv1.Result) Status {
	switch r {
	case runnerv1.Result_RESULT_SUCCESS:
		return StatusSuccess
	case runnerv1.Result_RESULT_FAILURE:
		return StatusFailure
	case runnerv1.Result_RESULT_CANCELLED:
		return StatusCancelled
	case runnerv1.Result_RESULT_SKIPPED:
		return StatusSkipped
	default:
		return StatusUnknown
	}
}
