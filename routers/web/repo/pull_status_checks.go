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

type statusCheckKind struct{ Name, strokeClass string }

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
			kind = statusCheckPending // shown with pending checks but counted separately
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

func (d *pullCommitStatusCheckData) allFinishedSuccessfully() bool {
	return d.count(statusCheckFailing, statusCheckPending, statusCheckInProgress, statusCheckExpected) == 0
}

func (d *pullCommitStatusCheckData) AllPassed() bool {
	return d.allFinishedSuccessfully() && d.RequireApprovalRunCount == 0
}

func (d *pullCommitStatusCheckData) Section(locale translation.Locale) *pullMergeBoxSection {
	parts := make([]string, 0, len(statusCheckKinds))
	for _, kind := range statusCheckKinds {
		if count := d.count(kind); count > 0 {
			parts = append(parts, string(locale.Tr("repo.pulls.status_checks_"+kind.Name, count)))
		}
	}
	section := &pullMergeBoxSection{
		StatusIcon: "octicon-check", StatusIconClass: "tw-bg-green", SectionTitle: locale.Tr("repo.pulls.status_checks_success"),
		SectionDetails: []template.HTML{locale.TrN(d.count(statusCheckKinds...), "repo.pulls.status_checks_count_1", "repo.pulls.status_checks_count_n", template.HTML(strings.Join(parts, ", ")))},
	}
	allFailed := d.RequireApprovalRunCount == 0 && len(d.Groups) == 1 && d.Groups[0].Kind == statusCheckFailing
	switch {
	case d.RequireApprovalRunCount > 0:
		section.StatusIcon, section.StatusIconClass = "octicon-alert-fill", "tw-bg-grey-light"
		section.SectionTitle = locale.TrN(d.RequireApprovalRunCount, "repo.pulls.status_checks_need_approvals_1", "repo.pulls.status_checks_need_approvals_n", d.RequireApprovalRunCount)
		section.SectionDetails = []template.HTML{locale.Tr("repo.pulls.status_checks_need_approvals_helper")}
	case allFailed:
		section.StatusIcon, section.StatusIconClass, section.SectionTitle = "octicon-x", "tw-bg-red", locale.Tr("repo.pulls.status_checks_all_failed")
	case d.count(statusCheckFailing) > 0:
		section.SectionTitle = locale.Tr("repo.pulls.status_checks_failure")
	case !d.allFinishedSuccessfully():
		section.SectionTitle = locale.Tr("repo.pulls.status_checking")
	}
	if !d.allFinishedSuccessfully() && !allFailed {
		section.ProgressSegments = d.progressSegments()
	}
	return section
}

type statusCheckProgressSegment struct {
	StrokeClass            string
	DashLength, DashOffset float64
}

// progressSegments splits a circle of length 100 into one arc per stroke color, separated by gaps
func (d *pullCommitStatusCheckData) progressSegments() (segments []statusCheckProgressSegment) {
	const gap = 5 // GitHub's 18 degree gaps
	total := float64(d.count(statusCheckKinds...))
	arcStart := gap / 2.0
	for _, group := range slices.Backward(d.Groups) {
		arcLength := 100 * float64(len(group.CommitStatuses)) / total
		last := len(segments) - 1
		if last >= 0 && segments[last].StrokeClass == group.Kind.strokeClass {
			segments[last].DashLength += arcLength
		} else {
			visibleLength := max(arcLength-gap, 0.01) // keep tiny arcs visible as a dot
			segments = append(segments, statusCheckProgressSegment{StrokeClass: group.Kind.strokeClass, DashLength: visibleLength, DashOffset: -arcStart})
		}
		arcStart += arcLength
	}
	if len(segments) == 1 {
		segments[0].DashLength = 100 // a single color needs no gap
	}
	return segments
}
