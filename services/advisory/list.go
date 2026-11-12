// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"slices"
	"strings"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/modules/util"
)

// ListFilters are the filters of the advisory lists of the web and the API, by the names used in the URL
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

// ParseListFilters returns the find options for the filters, without the repository and the viewer
func ParseListFilters(f ListFilters) (advisory_model.FindAdvisoriesOptions, error) {
	opts := advisory_model.FindAdvisoriesOptions{
		Keyword:     strings.TrimSpace(f.Keyword),
		Severity:    advisory_model.ParseSeverity(f.Severity),
		LabelIDs:    f.LabelIDs,
		Ecosystem:   f.Ecosystem,
		CweID:       strings.ToUpper(f.CweID),
		CloseReason: advisory_model.ParseCloseReason(f.CloseReason),
		SortBy:      f.SortBy,
		Ascending:   f.Direction == "asc",
	}
	if f.State != "" {
		state := advisory_model.ParseState(f.State)
		if state == 0 {
			return opts, util.NewInvalidArgumentErrorf("invalid state")
		}
		opts.States = []advisory_model.State{state}
	}
	if f.Severity != "" && opts.Severity == advisory_model.SeverityUnknown {
		return opts, util.NewInvalidArgumentErrorf("invalid severity")
	}
	if f.CloseReason != "" && opts.CloseReason == advisory_model.CloseReasonNone {
		return opts, util.NewInvalidArgumentErrorf("invalid close_reason")
	}
	if !slices.Contains([]string{"", "created", "updated", "published"}, f.SortBy) {
		return opts, util.NewInvalidArgumentErrorf("invalid sort")
	}
	if !slices.Contains([]string{"", "asc", "desc"}, f.Direction) {
		return opts, util.NewInvalidArgumentErrorf("invalid direction")
	}
	if opts.Ecosystem != "" && !slices.Contains(advisory_model.Ecosystems, opts.Ecosystem) {
		return opts, util.NewInvalidArgumentErrorf("invalid ecosystem")
	}
	if opts.CweID != "" && !IsValidCweID(opts.CweID) { // also keeps LIKE wildcards out
		return opts, util.NewInvalidArgumentErrorf("invalid cwe")
	}
	return opts, nil
}
