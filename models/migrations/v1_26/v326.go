// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

const actionsRunPath = "/actions/runs/"

type migrationRepository struct {
	ID        int64
	OwnerName string
	Name      string
}

type migrationActionRun struct {
	ID     int64
	RepoID int64
	Index  int64
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

func FixCommitStatusTargetURLToUseRunAndJobID(x *xorm.Engine) error {
	runByIndexCache := make(map[int64]map[int64]*migrationActionRun)
	jobsByRunIDCache := make(map[int64][]int64)
	repoLinkCache := make(map[int64]string)

	if err := migrateCommitStatusTargetURL(x, "commit_status", runByIndexCache, jobsByRunIDCache, repoLinkCache); err != nil {
		return err
	}
	return migrateCommitStatusTargetURL(x, "commit_status_summary", runByIndexCache, jobsByRunIDCache, repoLinkCache)
}

func migrateCommitStatusTargetURL(
	x *xorm.Engine,
	table string,
	runByIndexCache map[int64]map[int64]*migrationActionRun,
	jobsByRunIDCache map[int64][]int64,
	repoLinkCache map[int64]string,
) error {
	const batchSize = 500
	var lastID int64

	for {
		var rows []migrationCommitStatus
		sess := x.Table(table).
			Where("target_url LIKE ?", "%"+actionsRunPath+"%").
			And("id > ?", lastID).
			Asc("id").
			Limit(batchSize)
		if err := sess.Find(&rows); err != nil {
			return fmt.Errorf("query %s: %w", table, err)
		}
		if len(rows) == 0 {
			return nil
		}

		for _, row := range rows {
			lastID = row.ID
			if row.TargetURL == "" {
				continue
			}

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

			run, err := getRunByIndexCached(x, runByIndexCache, row.RepoID, runNum)
			if err != nil || run == nil {
				if err != nil {
					log.Warn("convert %s id=%d getRunByIndexCached: %v", table, row.ID, err)
				} else {
					log.Warn("convert %s id=%d: run not found for repo_id=%d run_index=%d", table, row.ID, row.RepoID, runNum)
				}
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
			newURL := fmt.Sprintf("%s%s%d/jobs/%d", repoLink, actionsRunPath, run.ID, jobID) // expect: {repo_link}/actions/runs/{run_id}/jobs/{job_id}
			if oldURL == newURL {
				continue
			}

			if _, err := x.Table(table).ID(row.ID).Cols("target_url").Update(&migrationCommitStatus{TargetURL: newURL}); err != nil {
				return fmt.Errorf("update %s id=%d target_url from %s to %s: %w", table, row.ID, oldURL, newURL, err)
			}
		}
	}
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

func getRunByIndexCached(x *xorm.Engine, cache map[int64]map[int64]*migrationActionRun, repoID, runIndex int64) (*migrationActionRun, error) {
	if repoCache, ok := cache[repoID]; ok {
		if run, ok := repoCache[runIndex]; ok {
			if run == nil {
				return nil, fmt.Errorf("run repo_id=%d run_index=%d not found", repoID, runIndex)
			}
			return run, nil
		}
	}

	var run migrationActionRun
	has, err := x.Table("action_run").Where("repo_id=? AND `index`=?", repoID, runIndex).Get(&run)
	if err != nil {
		return nil, err
	}
	if !has {
		if cache[repoID] == nil {
			cache[repoID] = make(map[int64]*migrationActionRun)
		}
		cache[repoID][runIndex] = nil
		return nil, fmt.Errorf("run repo_id=%d run_index=%d not found", repoID, runIndex)
	}
	if cache[repoID] == nil {
		cache[repoID] = make(map[int64]*migrationActionRun)
	}
	cache[repoID][runIndex] = &run
	return &run, nil
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

	parts := strings.Split(rest, "/") // expect: {run_num}/jobs/{job_num}
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
