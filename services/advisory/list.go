// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"slices"
	"strings"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/modules/util"
)

type ListFilters struct {
	State       string
	Keyword     string
	Severity    string
	LabelIDs    []int64
	Ecosystem   string
	CweID       string
	CloseReason string
	SortBy      string
	Direction   string
}

func ParseListFilters(filters ListFilters) (advisory_model.FindAdvisoriesOptions, error) {
	opts := advisory_model.FindAdvisoriesOptions{
		Keyword:     strings.TrimSpace(filters.Keyword),
		Severity:    advisory_model.ParseSeverity(filters.Severity),
		LabelIDs:    filters.LabelIDs,
		Ecosystem:   filters.Ecosystem,
		CweID:       strings.ToUpper(filters.CweID),
		CloseReason: advisory_model.ParseCloseReason(filters.CloseReason),
		SortBy:      filters.SortBy,
		Ascending:   filters.Direction == "asc",
	}
	if filters.State != "" {
		state := advisory_model.ParseState(filters.State)
		if state == 0 {
			return opts, util.NewInvalidArgumentErrorf("invalid state")
		}
		opts.States = []advisory_model.State{state}
	}
	if filters.Severity != "" && opts.Severity == advisory_model.SeverityUnknown {
		return opts, util.NewInvalidArgumentErrorf("invalid severity")
	}
	if filters.CloseReason != "" && opts.CloseReason == advisory_model.CloseReasonNone {
		return opts, util.NewInvalidArgumentErrorf("invalid close_reason")
	}
	if !slices.Contains([]string{"", "created", "updated", "published"}, filters.SortBy) {
		return opts, util.NewInvalidArgumentErrorf("invalid sort")
	}
	if !slices.Contains([]string{"", "asc", "desc"}, filters.Direction) {
		return opts, util.NewInvalidArgumentErrorf("invalid direction")
	}
	if opts.Ecosystem != "" && !slices.Contains(advisory_model.Ecosystems, opts.Ecosystem) {
		return opts, util.NewInvalidArgumentErrorf("invalid ecosystem")
	}
	if opts.CweID != "" && !cwePattern.MatchString(opts.CweID) { // also keeps LIKE wildcards out
		return opts, util.NewInvalidArgumentErrorf("invalid cwe")
	}
	return opts, nil
}
