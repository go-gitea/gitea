// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"fmt"
)

type IssueCloseReason int64

const (
	IssueCloseReasonNone IssueCloseReason = iota
	IssueCloseReasonCompleted
	IssueCloseReasonCompletedByCommit
	IssueCloseReasonCompletedByPull
	IssueCloseReasonAnswered
	IssueCloseReasonDuplicate
	IssueCloseReasonNotPlanned
)

func (r IssueCloseReason) String() string {
	switch r {
	case IssueCloseReasonCompleted:
		return "completed"
	case IssueCloseReasonCompletedByCommit:
		return "completed_by_commit"
	case IssueCloseReasonCompletedByPull:
		return "completed_by_pull"
	case IssueCloseReasonAnswered:
		return "answered"
	case IssueCloseReasonDuplicate:
		return "duplicate"
	case IssueCloseReasonNotPlanned:
		return "not_planned"
	default:
		return ""
	}
}

func (r IssueCloseReason) IsValid() bool {
	return r >= IssueCloseReasonNone && r <= IssueCloseReasonNotPlanned
}

func ParseIssueCloseReason(reason string) (IssueCloseReason, error) {
	switch reason {
	case "":
		return IssueCloseReasonNone, nil
	case "completed":
		return IssueCloseReasonCompleted, nil
	case "completed_by_commit":
		return IssueCloseReasonCompletedByCommit, nil
	case "completed_by_pull":
		return IssueCloseReasonCompletedByPull, nil
	case "answered":
		return IssueCloseReasonAnswered, nil
	case "duplicate":
		return IssueCloseReasonDuplicate, nil
	case "not_planned":
		return IssueCloseReasonNotPlanned, nil
	default:
		return IssueCloseReasonNone, fmt.Errorf("unknown close reason %q", reason)
	}
}

func parseIssueCloseReasonNumber(reason int64) (IssueCloseReason, error) {
	r := IssueCloseReason(reason)
	if !r.IsValid() {
		return IssueCloseReasonNone, fmt.Errorf("unknown close reason %d", reason)
	}
	return r, nil
}
