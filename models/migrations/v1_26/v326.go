// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"xorm.io/xorm"
)

const (
	actionsRunPath = "/actions/runs/"

	// Only commit status target URLs whose resolved run ID is smaller than this threshold are rewritten by this partial migration.
	// The fixed value 1000 is a conservative cutoff chosen to cover the smaller legacy run indexes that are most likely to be confused with ID-based URLs at runtime.
	// Larger legacy {run} or {job} numbers are usually easier to disambiguate. For example:
	//   * /actions/runs/1200/jobs/1420 is most likely an ID-based URL, because a run should not contain more than 256 jobs.
	//   * /actions/runs/1500/jobs/3 is most likely an index-based URL, because a job ID cannot be smaller than its run ID.
	// But URLs with small numbers, such as /actions/runs/5/jobs/6, are much harder to distinguish reliably.
	// This migration therefore prioritizes rewriting target URLs for runs in that lower range.
	legacyURLIDThreshold int64 = 1000
)

type migrationRepository struct {
	ID        int64
	OwnerName string
	Name      string
}

type migrationActionRun struct {
	ID           int64
	RepoID       int64
	Index        int64
	CommitSHA    string `xorm:"commit_sha"`
	Event        webhook_module.HookEventType
	TriggerEvent string
	EventPayload string
}

type migrationActionRunJob struct {
	ID    int64
	RunID int64
}

type migrationCommitStatus struct {
	ID        int64
	RepoID    int64
	TargetURL string
}

// Frozen subsets of modules/structs payload types, decoded from stored
// action_run.event_payload values. Inlined so the migration is insulated
// from future field changes in modules/structs.
type migrationPayloadCommit struct {
	ID string `json:"id"`
}

type migrationPushPayload struct {
	HeadCommit *migrationPayloadCommit `json:"head_commit"`
}

type migrationPRBranchInfo struct {
	Sha string `json:"sha"`
}

type migrationPullRequest struct {
	Head *migrationPRBranchInfo `json:"head"`
}

type migrationPullRequestPayload struct {
	PullRequest *migrationPullRequest `json:"pull_request"`
}

type commitSHAAndRuns struct {
	commitSHA string
	runs      map[int64]*migrationActionRun
}

// FixCommitStatusTargetURLToUseRunAndJobID partially migrates legacy Actions
// commit status target URLs to the new run/job ID-based form.
//
// Only rows whose resolved run ID is below legacyURLIDThreshold are rewritten.
// This is because smaller legacy run indexes are more likely to collide with run ID URLs during runtime resolution,
// so this migration prioritizes that lower range and leaves the remaining legacy target URLs to the web compatibility logic.
func FixCommitStatusTargetURLToUseRunAndJobID(x *xorm.Engine) error {
	jobsByRunIDCache := make(map[int64][]int64)
	repoLinkCache := make(map[int64]string)
	groups, err := loadLegacyMigrationRunGroups(x)
	if err != nil {
		return err
	}

	for repoID, groupsBySHA := range groups {
		for _, group := range groupsBySHA {
			if err := migrateCommitStatusTargetURLForGroup(x, "commit_status", repoID, group.commitSHA, group.runs, jobsByRunIDCache, repoLinkCache); err != nil {
				return err
			}
			if err := migrateCommitStatusTargetURLForGroup(x, "commit_status_summary", repoID, group.commitSHA, group.runs, jobsByRunIDCache, repoLinkCache); err != nil {
				return err
			}
		}
	}
	return nil
}

func loadLegacyMigrationRunGroups(x *xorm.Engine) (map[int64]map[string]*commitSHAAndRuns, error) {
	var runs []migrationActionRun
	if err := x.Table("action_run").
		Where("id < ?", legacyURLIDThreshold).
		Cols("id", "repo_id", "`index`", "commit_sha", "event", "trigger_event", "event_payload").
		Find(&runs); err != nil {
		return nil, fmt.Errorf("query action_run: %w", err)
	}

	groups := make(map[int64]map[string]*commitSHAAndRuns)
	for i := range runs {
		run := runs[i]
		commitID, err := getCommitStatusCommitID(&run)
		if err != nil {
			log.Warn("skip action_run id=%d when resolving commit status commit SHA: %v", run.ID, err)
			continue
		}
		if commitID == "" {
			// empty commitID means the run didn't create any commit status records, just skip
			continue
		}
		if groups[run.RepoID] == nil {
			groups[run.RepoID] = make(map[string]*commitSHAAndRuns)
		}
		if groups[run.RepoID][commitID] == nil {
			groups[run.RepoID][commitID] = &commitSHAAndRuns{
				commitSHA: commitID,
				runs:      make(map[int64]*migrationActionRun),
			}
		}
		groups[run.RepoID][commitID].runs[run.Index] = &run
	}
	return groups, nil
}

func migrateCommitStatusTargetURLForGroup(
	x *xorm.Engine,
	table string,
	repoID int64,
	sha string,
	runs map[int64]*migrationActionRun,
	jobsByRunIDCache map[int64][]int64,
	repoLinkCache map[int64]string,
) error {
	var rows []migrationCommitStatus
	if err := x.Table(table).
		Where("repo_id = ?", repoID).
		And("sha = ?", sha).
		Cols("id", "repo_id", "target_url").
		Find(&rows); err != nil {
		return fmt.Errorf("query %s for repo_id=%d sha=%s: %w", table, repoID, sha, err)
	}

	for _, row := range rows {
		repoLink, err := getRepoLinkCached(x, repoLinkCache, row.RepoID)
		if err != nil || repoLink == "" {
			if err != nil {
				log.Warn("convert %s id=%d getRepoLinkCached: %v", table, row.ID, err)
			} else {
				log.Warn("convert %s id=%d: repo=%d not found", table, row.ID, row.RepoID)
			}
			continue
		}

		runNum, jobNum, ok := parseTargetURL(row.TargetURL, repoLink)
		if !ok {
			continue
		}

		run, ok := runs[runNum]
		if !ok {
			continue
		}

		jobID, ok, err := getJobIDByIndexCached(x, jobsByRunIDCache, run.ID, jobNum)
		if err != nil || !ok {
			if err != nil {
				log.Warn("convert %s id=%d getJobIDByIndexCached: %v", table, row.ID, err)
			} else {
				log.Warn("convert %s id=%d: job not found for run_id=%d job_index=%d", table, row.ID, run.ID, jobNum)
			}
			continue
		}

		oldURL := row.TargetURL
		newURL := fmt.Sprintf("%s%s%d/jobs/%d", repoLink, actionsRunPath, run.ID, jobID)
		if oldURL == newURL {
			continue
		}

		if _, err := x.Table(table).ID(row.ID).Cols("target_url").Update(&migrationCommitStatus{TargetURL: newURL}); err != nil {
			return fmt.Errorf("update %s id=%d target_url from %s to %s: %w", table, row.ID, oldURL, newURL, err)
		}
	}
	return nil
}

func getRepoLinkCached(x *xorm.Engine, cache map[int64]string, repoID int64) (string, error) {
	if link, ok := cache[repoID]; ok {
		return link, nil
	}
	repo := &migrationRepository{}
	has, err := x.Table("repository").Where("id=?", repoID).Get(repo)
	if err != nil {
		return "", err
	}
	if !has {
		cache[repoID] = ""
		return "", nil
	}
	link := setting.AppSubURL + "/" + url.PathEscape(repo.OwnerName) + "/" + url.PathEscape(repo.Name)
	cache[repoID] = link
	return link, nil
}

func getJobIDByIndexCached(x *xorm.Engine, cache map[int64][]int64, runID, jobIndex int64) (int64, bool, error) {
	jobIDs, ok := cache[runID]
	if !ok {
		var jobs []migrationActionRunJob
		if err := x.Table("action_run_job").Where("run_id=?", runID).Asc("id").Cols("id").Find(&jobs); err != nil {
			return 0, false, err
		}
		jobIDs = make([]int64, 0, len(jobs))
		for _, job := range jobs {
			jobIDs = append(jobIDs, job.ID)
		}
		cache[runID] = jobIDs
	}
	if jobIndex < 0 || jobIndex >= int64(len(jobIDs)) {
		return 0, false, nil
	}
	return jobIDs[jobIndex], true, nil
}

func parseTargetURL(targetURL, repoLink string) (runNum, jobNum int64, ok bool) {
	prefix := repoLink + actionsRunPath
	if !strings.HasPrefix(targetURL, prefix) {
		return 0, 0, false
	}
	rest := targetURL[len(prefix):]

	parts := strings.Split(rest, "/")
	if len(parts) == 3 && parts[1] == "jobs" {
		runNum, err1 := strconv.ParseInt(parts[0], 10, 64)
		jobNum, err2 := strconv.ParseInt(parts[2], 10, 64)
		if err1 != nil || err2 != nil {
			return 0, 0, false
		}
		return runNum, jobNum, true
	}

	return 0, 0, false
}

func getCommitStatusCommitID(run *migrationActionRun) (string, error) {
	switch run.Event {
	case webhook_module.HookEventPush:
		payload, err := getPushEventPayload(run)
		if err != nil {
			return "", fmt.Errorf("getPushEventPayload: %w", err)
		}
		if payload.HeadCommit == nil {
			return "", errors.New("head commit is missing in event payload")
		}
		return payload.HeadCommit.ID, nil
	case webhook_module.HookEventPullRequest,
		webhook_module.HookEventPullRequestSync,
		webhook_module.HookEventPullRequestAssign,
		webhook_module.HookEventPullRequestLabel,
		webhook_module.HookEventPullRequestReviewRequest,
		webhook_module.HookEventPullRequestMilestone:
		payload, err := getPullRequestEventPayload(run)
		if err != nil {
			return "", fmt.Errorf("getPullRequestEventPayload: %w", err)
		}
		if payload.PullRequest == nil {
			return "", errors.New("pull request is missing in event payload")
		} else if payload.PullRequest.Head == nil {
			return "", errors.New("head of pull request is missing in event payload")
		}
		return payload.PullRequest.Head.Sha, nil
	case webhook_module.HookEventPullRequestReviewApproved,
		webhook_module.HookEventPullRequestReviewRejected,
		webhook_module.HookEventPullRequestReviewComment:
		payload, err := getPullRequestEventPayload(run)
		if err != nil {
			return "", fmt.Errorf("getPullRequestEventPayload: %w", err)
		}
		if payload.PullRequest == nil {
			return "", errors.New("pull request is missing in event payload")
		} else if payload.PullRequest.Head == nil {
			return "", errors.New("head of pull request is missing in event payload")
		}
		return payload.PullRequest.Head.Sha, nil
	case webhook_module.HookEventRelease:
		return run.CommitSHA, nil
	default:
		return "", nil
	}
}

func getPushEventPayload(run *migrationActionRun) (*migrationPushPayload, error) {
	if run.Event != webhook_module.HookEventPush {
		return nil, fmt.Errorf("event %s is not a push event", run.Event)
	}
	var payload migrationPushPayload
	if err := json.Unmarshal([]byte(run.EventPayload), &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func getPullRequestEventPayload(run *migrationActionRun) (*migrationPullRequestPayload, error) {
	if !run.Event.IsPullRequest() && !run.Event.IsPullRequestReview() {
		return nil, fmt.Errorf("event %s is not a pull request event", run.Event)
	}
	var payload migrationPullRequestPayload
	if err := json.Unmarshal([]byte(run.EventPayload), &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}
