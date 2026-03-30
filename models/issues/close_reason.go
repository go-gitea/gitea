// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"database/sql/driver"
	"encoding/json"
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

func (r IssueCloseReason) Value() (driver.Value, error) {
	if !r.IsValid() {
		return nil, fmt.Errorf("unknown close reason %d", r)
	}
	return r.String(), nil
}

func (r *IssueCloseReason) Scan(src any) error {
	if src == nil {
		*r = IssueCloseReasonNone
		return nil
	}

	switch v := src.(type) {
	case string:
		parsed, err := ParseIssueCloseReason(v)
		if err != nil {
			return err
		}
		*r = parsed
		return nil
	case []byte:
		return r.Scan(string(v))
	case int64:
		parsed, err := parseIssueCloseReasonNumber(v)
		if err != nil {
			return err
		}
		*r = parsed
		return nil
	case int:
		return r.Scan(int64(v))
	default:
		return fmt.Errorf("unsupported close reason type %T", src)
	}
}

func (r IssueCloseReason) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.String())
}

func (r *IssueCloseReason) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*r = IssueCloseReasonNone
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parsed, parseErr := ParseIssueCloseReason(s)
		if parseErr != nil {
			return parseErr
		}
		*r = parsed
		return nil
	}

	var n int64
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	parsed, err := parseIssueCloseReasonNumber(n)
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}
