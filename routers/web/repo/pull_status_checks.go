// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"html/template"
	"slices"
	"strings"

	actions_model "gitea.dev/models/actions"
	git_model "gitea.dev/models/git"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/base"
	"gitea.dev/modules/translation"
)

type statusCheckKind struct{ Name, ringColor string }

var (
	statusCheckFailing    = statusCheckKind{"failing", "tw-stroke-red"}
	statusCheckPending    = statusCheckKind{"pending", "tw-stroke-yellow"}
	statusCheckInProgress = statusCheckKind{"in_progress", "tw-stroke-yellow"}
	statusCheckExpected   = statusCheckKind{"expected", "tw-stroke-yellow"}
	statusCheckSkipped    = statusCheckKind{"skipped", "tw-stroke-grey-light"}
	statusCheckSuccessful = statusCheckKind{"successful", "tw-stroke-green"}
)

var statusCheckKinds = []statusCheckKind{statusCheckFailing, statusCheckPending, statusCheckInProgress, statusCheckSkipped, statusCheckExpected, statusCheckSuccessful}

type statusCheckGroup struct {
	Kind           statusCheckKind
	CommitStatuses []*git_model.CommitStatus
}

func (g *statusCheckGroup) Title(locale translation.Locale) template.HTML {
	return locale.TrN(len(g.CommitStatuses), "repo.pulls.status_checks_count_1", "repo.pulls.status_checks_count_n", locale.Tr("repo.pulls.status_checks_"+g.Kind.Name, len(g.CommitStatuses)))
}

func statusCheckKindOf(cs *git_model.CommitStatus, actionsStatuses actions_module.CommitActionsStatusMap) statusCheckKind {
	switch {
	case cs.State.IsError() || cs.State.IsFailure() || cs.State.IsWarning():
		return statusCheckFailing
	case cs.ID == 0:
		return statusCheckExpected
	case cs.State.IsPending() && actionsStatuses[cs.ID] == actions_model.StatusRunning:
		return statusCheckInProgress
	case cs.State.IsPending():
		return statusCheckPending
	case cs.State.IsSuccess():
		return statusCheckSuccessful
	default:
		return statusCheckSkipped
	}
}

func (d *pullCommitStatusCheckData) groupStatuses(commitStatuses []*git_model.CommitStatus) {
	d.counts = map[statusCheckKind]int{}
	byKind := map[statusCheckKind][]*git_model.CommitStatus{}
	for _, cs := range commitStatuses {
		kind := statusCheckKindOf(cs, d.ActionsStatuses)
		d.counts[kind]++
		d.lastUpdated = max(d.lastUpdated, cs.UpdatedUnix)
		if kind == statusCheckExpected {
			kind = statusCheckPending // listed with the pending checks but counted separately, like GitHub
		}
		byKind[kind] = append(byKind[kind], cs)
	}
	for _, kind := range statusCheckKinds {
		if statuses := byKind[kind]; len(statuses) > 0 {
			slices.SortStableFunc(statuses, func(a, b *git_model.CommitStatus) int { return base.NaturalSortCompare(a.Context, b.Context) })
			d.Groups = append(d.Groups, &statusCheckGroup{Kind: kind, CommitStatuses: statuses})
		}
	}
}

func (d *pullCommitStatusCheckData) count(kinds ...statusCheckKind) (count int) {
	for _, kind := range kinds {
		count += d.counts[kind]
	}
	return count
}

func (d *pullCommitStatusCheckData) AllPassed() bool {
	return d.count(statusCheckFailing, statusCheckPending, statusCheckInProgress, statusCheckExpected) == 0
}

// Section is the checks header, with a ring unless all checks agree
func (d *pullCommitStatusCheckData) Section(locale translation.Locale) *pullMergeBoxSection {
	parts := make([]string, 0, len(statusCheckKinds))
	for _, kind := range statusCheckKinds {
		if count := d.count(kind); count > 0 {
			parts = append(parts, string(locale.Tr("repo.pulls.status_checks_"+kind.Name, count)))
		}
	}
	section := &pullMergeBoxSection{
		Icon: "octicon-check", IconClass: "tw-bg-green", Title: locale.Tr("repo.pulls.status_checks_success"),
		Details: []template.HTML{locale.TrN(d.count(statusCheckKinds...), "repo.pulls.status_checks_count_1", "repo.pulls.status_checks_count_n", template.HTML(strings.Join(parts, ", ")))},
	}
	allFailed := d.RequireApprovalRunCount == 0 && len(d.Groups) == 1 && d.Groups[0].Kind == statusCheckFailing
	switch {
	case d.RequireApprovalRunCount > 0:
		section.Title = locale.TrN(d.RequireApprovalRunCount, "repo.pulls.status_checks_need_approvals_1", "repo.pulls.status_checks_need_approvals_n", d.RequireApprovalRunCount)
		section.Details = []template.HTML{locale.Tr("repo.pulls.status_checks_need_approvals_helper")}
	case allFailed:
		section.Icon, section.IconClass, section.Title = "octicon-x", "tw-bg-red", locale.Tr("repo.pulls.status_checks_all_failed")
	case d.count(statusCheckFailing) > 0:
		section.Title = locale.Tr("repo.pulls.status_checks_failure")
	case !d.AllPassed():
		section.Title = locale.Tr("repo.pulls.status_checking")
	}
	if !d.AllPassed() && !allFailed {
		section.Ring = d.ringSegments()
	}
	return section
}

type statusCheckRingSegment struct {
	ColorClass   string
	Dash, Offset float64
}

// ringSegments splits a 100-length ring into one arc per group, with GitHub's 18 degree gaps
func (d *pullCommitStatusCheckData) ringSegments() (segments []statusCheckRingSegment) {
	const gap = 5
	total, start := float64(d.count(statusCheckKinds...)), gap/2.0
	for _, group := range slices.Backward(d.Groups) {
		length := 100 * float64(len(group.CommitStatuses)) / total
		if last := len(segments) - 1; last >= 0 && segments[last].ColorClass == group.Kind.ringColor {
			segments[last].Dash += length
		} else {
			segments = append(segments, statusCheckRingSegment{ColorClass: group.Kind.ringColor, Dash: max(length-gap, 0.01), Offset: -start})
		}
		start += length
	}
	if len(segments) == 1 {
		segments[0].Dash = 100
	}
	return segments
}
