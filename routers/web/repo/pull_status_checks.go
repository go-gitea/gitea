// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"strings"

	actions_model "gitea.dev/models/actions"
	git_model "gitea.dev/models/git"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/translation"
)

// StatusCheckGroupKind classifies a commit status for display in the PR merge box's checks widget.
type StatusCheckGroupKind string

const (
	StatusCheckGroupFailed     StatusCheckGroupKind = "failed"
	StatusCheckGroupPending    StatusCheckGroupKind = "pending"
	StatusCheckGroupQueued     StatusCheckGroupKind = "queued"
	StatusCheckGroupInProgress StatusCheckGroupKind = "in_progress"
	StatusCheckGroupSkipped    StatusCheckGroupKind = "skipped"
	StatusCheckGroupSuccess    StatusCheckGroupKind = "success"
)

// section maps queued checks into GitHub's combined pending section while preserving running checks separately.
func (k StatusCheckGroupKind) section() StatusCheckGroupKind {
	if k == StatusCheckGroupQueued {
		return StatusCheckGroupPending
	}
	return k
}

// StatusCheckGroup is one collapsible section of the checks widget.
type StatusCheckGroup struct {
	Kind                  StatusCheckGroupKind
	CommitStatuses        []*git_model.CommitStatus
	MissingRequiredChecks []string // only ever non-empty on the Pending group
}

// Count is the number of checks shown in this group, used by the group header.
func (g *StatusCheckGroup) Count() int {
	return len(g.CommitStatuses) + len(g.MissingRequiredChecks)
}

// statusCheckSectionOrder is the fixed display order of sections, most-attention-worthy first.
var statusCheckSectionOrder = []StatusCheckGroupKind{
	StatusCheckGroupFailed,
	StatusCheckGroupPending,
	StatusCheckGroupInProgress,
	StatusCheckGroupSkipped,
	StatusCheckGroupSuccess,
}

// statusCheckSummaryOrder is the fixed order of the fine-grained summary line fragments.
var statusCheckSummaryOrder = []StatusCheckGroupKind{
	StatusCheckGroupFailed,
	StatusCheckGroupPending,
	StatusCheckGroupInProgress,
	StatusCheckGroupQueued,
	StatusCheckGroupSkipped,
	StatusCheckGroupSuccess,
}

// classifyCommitStatus decides which fine-grained StatusCheckGroupKind a commit status belongs to. When the
// status is backed by a Gitea Actions job, the live actions_model.Status (from actionsStatusMap) is preferred
// over the coarser stored commitstatus.CommitStatusState, since it distinguishes queued/running/blocked.
func classifyCommitStatus(cs *git_model.CommitStatus, actionsStatusMap actions_module.CommitActionsStatusMap) StatusCheckGroupKind {
	if actionStatus, ok := actionsStatusMap[cs.ID]; ok {
		switch {
		case actionStatus.In(actions_model.StatusFailure, actions_model.StatusCancelled, actions_model.StatusCancelling):
			return StatusCheckGroupFailed
		case actionStatus == actions_model.StatusRunning:
			return StatusCheckGroupInProgress
		case actionStatus.In(actions_model.StatusWaiting, actions_model.StatusBlocked):
			return StatusCheckGroupQueued
		case actionStatus == actions_model.StatusSkipped:
			return StatusCheckGroupSkipped
		case actionStatus == actions_model.StatusSuccess:
			return StatusCheckGroupSuccess
		}
	}

	switch cs.State {
	case commitstatus.CommitStatusError, commitstatus.CommitStatusFailure:
		return StatusCheckGroupFailed
	case commitstatus.CommitStatusSkipped, commitstatus.CommitStatusWarning:
		return StatusCheckGroupSkipped
	case commitstatus.CommitStatusSuccess:
		return StatusCheckGroupSuccess
	default: // commitstatus.CommitStatusPending
		return StatusCheckGroupPending
	}
}

// buildStatusCheckGroups classifies commitStatuses (plus any missingRequiredChecks, which always count as
// Pending) and returns the sections and summary counts used by the checks widget.
func buildStatusCheckGroups(commitStatuses []*git_model.CommitStatus, actionsStatusMap actions_module.CommitActionsStatusMap, missingRequiredChecks []string) (groups []*StatusCheckGroup, summaryCounts map[StatusCheckGroupKind]int) {
	sections := make(map[StatusCheckGroupKind]*StatusCheckGroup, len(statusCheckSectionOrder))
	for _, kind := range statusCheckSectionOrder {
		sections[kind] = &StatusCheckGroup{Kind: kind}
	}
	summaryCounts = make(map[StatusCheckGroupKind]int, len(statusCheckSummaryOrder))

	for _, cs := range commitStatuses {
		kind := classifyCommitStatus(cs, actionsStatusMap)
		summaryCounts[kind]++
		sections[kind.section()].CommitStatuses = append(sections[kind.section()].CommitStatuses, cs)
	}
	summaryCounts[StatusCheckGroupPending] += len(missingRequiredChecks)
	sections[StatusCheckGroupPending].MissingRequiredChecks = missingRequiredChecks

	groups = make([]*StatusCheckGroup, 0, len(statusCheckSectionOrder))
	for _, kind := range statusCheckSectionOrder {
		section := sections[kind]
		if section.Count() > 0 {
			groups = append(groups, section)
		}
	}
	return groups, summaryCounts
}

// statusCheckSummaryLocaleKey maps a fine-grained kind to the locale key for its "%d pending"-style summary fragment.
var statusCheckSummaryLocaleKey = map[StatusCheckGroupKind]string{
	StatusCheckGroupFailed:     "repo.pulls.status_checks_summary_failed",
	StatusCheckGroupPending:    "repo.pulls.status_checks_summary_pending",
	StatusCheckGroupQueued:     "repo.pulls.status_checks_summary_queued",
	StatusCheckGroupInProgress: "repo.pulls.status_checks_summary_in_progress",
	StatusCheckGroupSkipped:    "repo.pulls.status_checks_summary_skipped",
	StatusCheckGroupSuccess:    "repo.pulls.status_checks_summary_success",
}

// Summary renders the "N pending, M queued, ... checks" line shown under the checks widget's headline.
func (d *PullCommitStatusCheckData) Summary(locale translation.Locale) string {
	parts := make([]string, 0, len(statusCheckSummaryOrder))
	for _, kind := range statusCheckSummaryOrder {
		if count := d.SummaryCounts[kind]; count > 0 {
			parts = append(parts, locale.TrString(statusCheckSummaryLocaleKey[kind], count))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return locale.TrString("repo.pulls.status_checks_summary_checks", strings.Join(parts, ", "))
}

// HasPendingChecks reports whether the checks widget contains work that has not completed yet.
func (d *PullCommitStatusCheckData) HasPendingChecks() bool {
	return d.SummaryCounts[StatusCheckGroupPending] > 0 ||
		d.SummaryCounts[StatusCheckGroupQueued] > 0 ||
		d.SummaryCounts[StatusCheckGroupInProgress] > 0
}
