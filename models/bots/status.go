// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

// Status represents the status of Run, RunJob, Task, or TaskStep
type Status int

const (
	StatusUnknown   Status = iota // 0, consistent with runnerv1.Result_RESULT_UNSPECIFIED
	StatusSuccess                 // 1, consistent with runnerv1.Result_RESULT_SUCCESS
	StatusFailure                 // 2, consistent with runnerv1.Result_RESULT_FAILURE
	StatusCancelled               // 3, consistent with runnerv1.Result_RESULT_CANCELLED
	StatusSkipped                 // 4, consistent with runnerv1.Result_RESULT_SKIPPED
	StatusWaiting                 // 5
	StatusRunning                 // 6
)

// String returns the string name of the Status
func (s Status) String() string {
	return statusNames[s]
}

// IsDone returns whether the Status is final
func (s Status) IsDone() bool {
	return s > StatusUnknown && s <= StatusSkipped
}

var statusNames = map[Status]string{
	StatusUnknown:   "unknown",
	StatusWaiting:   "waiting",
	StatusRunning:   "running",
	StatusSuccess:   "success",
	StatusFailure:   "failure",
	StatusCancelled: "cancelled",
	StatusSkipped:   "skipped",
}
