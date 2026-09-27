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
	"gitea.dev/modules/translation"
)

type statusCheckKind struct{ name, ringColor string }

var (
	statusCheckFailing    = statusCheckKind{"failing", "tw-stroke-red"}
	statusCheckPending    = statusCheckKind{"pending", "tw-stroke-yellow"}
	statusCheckInProgress = statusCheckKind{"in_progress", "tw-stroke-yellow"}
	statusCheckExpected   = statusCheckKind{"expected", "tw-stroke-yellow"}
	statusCheckSkipped    = statusCheckKind{"skipped", "tw-stroke-grey-light"}
	statusCheckSuccessful = statusCheckKind{"successful", "tw-stroke-green"}
)

var statusCheckKinds = []statusCheckKind{statusCheckFailing, statusCheckPending, statusCheckInProgress, statusCheckExpected, statusCheckSkipped, statusCheckSuccessful}

type statusCheckGroup struct {
	kind           statusCheckKind
	CommitStatuses []*git_model.CommitStatus
}

func (g *statusCheckGroup) text(locale translation.Locale) template.HTML {
	return locale.Tr("repo.pulls.status_checks_"+g.kind.name, len(g.CommitStatuses))
}

func (g *statusCheckGroup) Title(locale translation.Locale) template.HTML {
	return locale.TrN(len(g.CommitStatuses), "repo.pulls.status_checks_count_1", "repo.pulls.status_checks_count_n", g.text(locale))
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

func groupStatusChecks(commitStatuses []*git_model.CommitStatus, actionsStatuses actions_module.CommitActionsStatusMap) (groups []*statusCheckGroup) {
	for _, kind := range statusCheckKinds {
		group := &statusCheckGroup{kind: kind}
		for _, cs := range commitStatuses {
			if statusCheckKindOf(cs, actionsStatuses) == kind {
				group.CommitStatuses = append(group.CommitStatuses, cs)
			}
		}
		if len(group.CommitStatuses) > 0 {
			slices.SortStableFunc(group.CommitStatuses, func(a, b *git_model.CommitStatus) int { return strings.Compare(a.Context, b.Context) })
			groups = append(groups, group)
		}
	}
	return groups
}

func (d *pullCommitStatusCheckData) count(kinds ...statusCheckKind) (count int) {
	for _, group := range d.Groups {
		if slices.Contains(kinds, group.kind) {
			count += len(group.CommitStatuses)
		}
	}
	return count
}

func (d *pullCommitStatusCheckData) AllPassed() bool {
	return d.count(statusCheckFailing, statusCheckPending, statusCheckInProgress, statusCheckExpected) == 0
}

func (d *pullCommitStatusCheckData) hasPending() bool {
	return d.count(statusCheckPending, statusCheckInProgress) > 0
}

// Section is the collapsible header of the checks, a solid circle when all checks agree, otherwise a ring
func (d *pullCommitStatusCheckData) Section(locale translation.Locale) *pullMergeBoxSection {
	parts := make([]string, 0, len(d.Groups))
	for _, group := range d.Groups {
		parts = append(parts, string(group.text(locale)))
	}
	section := &pullMergeBoxSection{
		Icon: "octicon-check", IconClass: "tw-bg-green", Title: locale.Tr("repo.pulls.status_checks_success"),
		Details: []template.HTML{locale.TrN(d.count(statusCheckKinds...), "repo.pulls.status_checks_count_1", "repo.pulls.status_checks_count_n", template.HTML(strings.Join(parts, ", ")))},
	}
	allFailed := len(d.Groups) == 1 && d.Groups[0].kind == statusCheckFailing
	switch {
	case allFailed:
		section.Icon, section.IconClass, section.Title = "octicon-x", "tw-bg-red", locale.Tr("repo.pulls.status_checks_all_failed")
	case d.count(statusCheckFailing) > 0:
		section.Title = locale.Tr("repo.pulls.status_checks_failure")
	case d.RequireApprovalRunCount > 0:
		section.Title = locale.Tr("repo.pulls.status_checks_waiting_for_approval")
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

// ringSegments splits a ring with a path length of 100 into one arc per group, with GitHub's 18 degree gaps
func (d *pullCommitStatusCheckData) ringSegments() (segments []statusCheckRingSegment) {
	const gap = 5
	total, start := float64(d.count(statusCheckKinds...)), gap/2.0
	for _, group := range slices.Backward(d.Groups) {
		length := 100 * float64(len(group.CommitStatuses)) / total
		if last := len(segments) - 1; last >= 0 && segments[last].ColorClass == group.kind.ringColor {
			segments[last].Dash += length
		} else {
			segments = append(segments, statusCheckRingSegment{ColorClass: group.kind.ringColor, Dash: max(length-gap, 0.01), Offset: -start})
		}
		start += length
	}
	if len(segments) == 1 {
		segments[0].Dash = 100
	}
	return segments
}
