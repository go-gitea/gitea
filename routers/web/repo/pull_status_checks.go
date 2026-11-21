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

type statusCheckKind struct{ KindName, countTrKey, strokeClass string }

var (
	statusCheckFailing    = statusCheckKind{"failing", "repo.pulls.status_checks_failing", "tw-stroke-red"}
	statusCheckPending    = statusCheckKind{"pending", "repo.pulls.status_checks_pending", "tw-stroke-yellow"}
	statusCheckInProgress = statusCheckKind{"in_progress", "repo.pulls.status_checks_in_progress", "tw-stroke-yellow"}
	statusCheckExpected   = statusCheckKind{"expected", "repo.pulls.status_checks_expected", "tw-stroke-yellow"}
	statusCheckSkipped    = statusCheckKind{"skipped", "repo.pulls.status_checks_skipped", "tw-stroke-grey-light"}
	statusCheckSuccessful = statusCheckKind{"successful", "repo.pulls.status_checks_successful", "tw-stroke-green"}
)

var statusCheckKinds = []statusCheckKind{statusCheckFailing, statusCheckPending, statusCheckInProgress, statusCheckSkipped, statusCheckExpected, statusCheckSuccessful}

type pullStatusCheck struct {
	StatusContext, StatusDescription, StatusTargetURL string
	StatusState                                       commitstatus.CommitStatusState
	ActionStatus                                      string
	IsRequired                                        bool
}

type statusCheckGroup struct {
	CheckKind    statusCheckKind
	StatusChecks []*pullStatusCheck
}

func (g *statusCheckGroup) GroupTitle(locale translation.Locale) template.HTML {
	return locale.TrN(len(g.StatusChecks), "repo.pulls.status_checks_count_1", "repo.pulls.status_checks_count_n", locale.Tr(g.CheckKind.countTrKey, len(g.StatusChecks)))
}

func (d *pullCommitStatusCheckData) groupStatuses(commitStatuses []*git_model.CommitStatus, requiredContexts []string, actionsStatuses actions_module.CommitActionsStatusMap, locale translation.Locale) {
	d.counts = map[statusCheckKind]int{}
	byKind := map[statusCheckKind][]*pullStatusCheck{}
	checks := make([]*pullStatusCheck, len(commitStatuses))
	for i, cs := range commitStatuses {
		checks[i] = &pullStatusCheck{
			StatusContext: cs.Context, StatusDescription: cs.Description, StatusTargetURL: cs.TargetURL,
			StatusState: cs.State, ActionStatus: actionsStatuses.IconStatus(cs),
		}
		kind := statusCheckPending // unknown states count as pending, like in the combined status
		switch {
		case cs.State.IsError() || cs.State.IsFailure() || cs.State.IsWarning():
			kind = statusCheckFailing
		case cs.State == commitstatus.CommitStatusRunning || actionsStatuses[cs.ID] == actions_model.StatusRunning && cs.State.IsPending():
			kind = statusCheckInProgress
		case cs.State.IsSuccess():
			kind = statusCheckSuccessful
		case cs.State.IsSkipped():
			kind = statusCheckSkipped
		}
		d.counts[kind]++
		byKind[kind] = append(byKind[kind], checks[i])
	}
	for _, requiredContext := range requiredContexts {
		matchesRequiredContext := createRequiredContextMatcher(requiredContext)
		contextFound := false
		for _, check := range checks {
			if matchesRequiredContext(check.StatusContext) {
				check.IsRequired = true
				contextFound = true
			}
		}
		if !contextFound {
			d.counts[statusCheckExpected]++
			byKind[statusCheckPending] = append(byKind[statusCheckPending], &pullStatusCheck{
				StatusContext: requiredContext, StatusDescription: locale.TrString("repo.pulls.status_checks_waiting"),
				StatusState: commitstatus.CommitStatusPending, IsRequired: true,
			})
		}
	}
	for _, kind := range statusCheckKinds {
		if checks := byKind[kind]; len(checks) > 0 {
			slices.SortStableFunc(checks, func(first, second *pullStatusCheck) int {
				return base.NaturalSortCompare(first.StatusContext, second.StatusContext)
			})
			d.StatusCheckGroups = append(d.StatusCheckGroups, &statusCheckGroup{CheckKind: kind, StatusChecks: checks})
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

func (d *pullCommitStatusCheckData) StatusChecksSection(locale translation.Locale) *pullMergeBoxSection {
	parts := make([]string, 0, len(statusCheckKinds))
	for _, kind := range statusCheckKinds {
		if count := d.count(kind); count > 0 {
			parts = append(parts, string(locale.Tr(kind.countTrKey, count)))
		}
	}
	section := &pullMergeBoxSection{
		StatusIcon: "octicon-check", StatusIconClass: "tw-bg-green", SectionTitle: locale.Tr("repo.pulls.status_checks_success"),
		SectionDetails: []template.HTML{locale.TrN(d.count(statusCheckKinds...), "repo.pulls.status_checks_count_1", "repo.pulls.status_checks_count_n", template.HTML(strings.Join(parts, ", ")))},
	}
	allFailed := d.RequireApprovalRunCount == 0 && len(d.StatusCheckGroups) == 1 && d.StatusCheckGroups[0].CheckKind == statusCheckFailing
	switch {
	case d.RequireApprovalRunCount > 0:
		section.StatusIcon, section.StatusIconClass = "octicon-alert-fill", "tw-bg-grey-light"
		section.SectionTitle = locale.TrN(d.RequireApprovalRunCount, "repo.pulls.status_checks_need_approvals", "repo.pulls.status_checks_need_approvals_n", d.RequireApprovalRunCount)
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

func (d *pullCommitStatusCheckData) progressSegments() (segments []statusCheckProgressSegment) {
	const segmentGap = 5
	total, start := float64(d.count(statusCheckKinds...)), segmentGap/2.0
	for _, kinds := range [][]statusCheckKind{{statusCheckSuccessful}, {statusCheckSkipped}, {statusCheckPending, statusCheckInProgress, statusCheckExpected}, {statusCheckFailing}} {
		count := d.count(kinds...)
		if count == 0 {
			continue
		}
		length := 100 * float64(count) / total
		segments = append(segments, statusCheckProgressSegment{StrokeClass: kinds[0].strokeClass, DashLength: max(length-segmentGap, 0), DashOffset: -start})
		start += length
	}
	if len(segments) == 1 {
		segments[0].DashLength, segments[0].DashOffset = 100, 0
	}
	return segments
}
