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
	"gitea.dev/modules/commitstatus"
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

type pullStatusCheck struct {
	Context, Description, TargetURL string
	State                           commitstatus.CommitStatusState
	Kind                            statusCheckKind
	ActionStatus                    string
	IsRequired                      bool
}

type statusCheckGroup struct {
	Kind   statusCheckKind
	Checks []*pullStatusCheck
}

func (g *statusCheckGroup) Title(locale translation.Locale) template.HTML {
	return locale.TrN(len(g.Checks), "repo.pulls.status_checks_count_1", "repo.pulls.status_checks_count_n", locale.Tr("repo.pulls.status_checks_"+g.Kind.Name, len(g.Checks)))
}

func statusCheckKindOf(cs *git_model.CommitStatus, actionsStatuses actions_module.CommitActionsStatusMap) statusCheckKind {
	switch {
	case cs.State.IsError() || cs.State.IsFailure() || cs.State.IsWarning():
		return statusCheckFailing
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

func (d *pullCommitStatusCheckData) groupStatuses(commitStatuses []*git_model.CommitStatus, requiredContexts []string, actionsStatuses actions_module.CommitActionsStatusMap, locale translation.Locale) {
	checks := make([]*pullStatusCheck, len(commitStatuses))
	for i, cs := range commitStatuses {
		checks[i] = &pullStatusCheck{
			Context: cs.Context, Description: cs.Description, TargetURL: cs.TargetURL,
			State: cs.State, Kind: statusCheckKindOf(cs, actionsStatuses), ActionStatus: actionsStatuses.IconStatus(cs),
		}
	}
	for _, requiredContext := range requiredContexts {
		matchesRequiredContext := createRequiredContextMatcher(requiredContext)
		contextFound := false
		for i, cs := range commitStatuses {
			if matchesRequiredContext(cs.Context) {
				checks[i].IsRequired = true
				contextFound = true
			}
		}
		if !contextFound {
			checks = append(checks, &pullStatusCheck{
				Context: requiredContext, Description: locale.TrString("repo.pulls.status_checks_waiting"),
				State: commitstatus.CommitStatusPending, Kind: statusCheckExpected, IsRequired: true,
			})
		}
	}
	d.counts = map[statusCheckKind]int{}
	byKind := map[statusCheckKind][]*pullStatusCheck{}
	for _, check := range checks {
		d.counts[check.Kind]++
		kind := check.Kind
		if kind == statusCheckExpected {
			kind = statusCheckPending
		}
		byKind[kind] = append(byKind[kind], check)
	}
	for _, kind := range statusCheckKinds {
		if checks := byKind[kind]; len(checks) > 0 {
			slices.SortStableFunc(checks, func(first, second *pullStatusCheck) int {
				return base.NaturalSortCompare(first.Context, second.Context)
			})
			d.Groups = append(d.Groups, &statusCheckGroup{Kind: kind, Checks: checks})
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
		section.SectionDetails = []template.HTML{locale.Tr("repo.pulls.status_checks_approval_required_desc")}
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

func (d *pullCommitStatusCheckData) progressSegments() (segments []statusCheckProgressSegment) {
	const segmentGap = 5
	total, start := float64(d.count(statusCheckKinds...)), segmentGap/2.0
	for _, kind := range []statusCheckKind{statusCheckSuccessful, statusCheckSkipped, statusCheckPending, statusCheckFailing} {
		count := d.count(kind)
		if kind == statusCheckPending {
			count += d.count(statusCheckInProgress, statusCheckExpected)
		}
		if count == 0 {
			continue
		}
		length := 100 * float64(count) / total
		segments = append(segments, statusCheckProgressSegment{StrokeClass: kind.strokeClass, DashLength: max(length-segmentGap, 0), DashOffset: -start})
		start += length
	}
	if len(segments) == 1 {
		segments[0].DashLength, segments[0].DashOffset = 100, 0
	}
	return segments
}
