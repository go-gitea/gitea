// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const (
	actionsRunPath                    = "/actions/runs/"
	fixCommitStatusTargetURLBatchSize = 500
)

func init() {
	Register(&Check{
		Title:     "Fix legacy Actions commit status target URLs (Slow fix, see #37008)",
		Name:      "fix-commit-status-target-url",
		IsDefault: false,
		Run:       fixCommitStatusTargetURL,
		Priority:  9,
	})
}

type commitStatusTargetURLRow struct {
	ID        int64
	RepoID    int64
	TargetURL string
}

type targetURLFixStats struct {
	scanned   int
	matched   int
	fixable   int
	updated   int
	unfixable int
}

func fixCommitStatusTargetURL(ctx context.Context, logger log.Logger, autofix bool) error {
	repoLinkCache := make(map[int64]string)
	runByIndexCache := make(map[int64]map[int64]*actions_model.ActionRun)
	jobsByRunIDCache := make(map[int64][]int64)

	tables := []string{"commit_status", "commit_status_summary"}
	total := targetURLFixStats{}
	for _, table := range tables {
		stats, err := fixCommitStatusTargetURLForTable(ctx, logger, autofix, table, repoLinkCache, runByIndexCache, jobsByRunIDCache)
		if err != nil {
			return err
		}
		total.scanned += stats.scanned
		total.matched += stats.matched
		total.fixable += stats.fixable
		total.updated += stats.updated
		total.unfixable += stats.unfixable
	}

	if total.matched == 0 {
		logger.Info("Found no commit status target URLs containing %q", actionsRunPath)
		return nil
	}

	if autofix {
		logger.Info("Scanned %d rows with %d Actions target URLs; updated %d rows, %d unfixable.", total.scanned, total.matched, total.updated, total.unfixable)
	} else {
		logger.Warn("Scanned %d rows with %d Actions target URLs; found %d fixable rows, %d unfixable.", total.scanned, total.matched, total.fixable, total.unfixable)
	}

	return nil
}

func fixCommitStatusTargetURLForTable(
	ctx context.Context,
	logger log.Logger,
	autofix bool,
	table string,
	repoLinkCache map[int64]string,
	runByIndexCache map[int64]map[int64]*actions_model.ActionRun,
	jobsByRunIDCache map[int64][]int64,
) (targetURLFixStats, error) {
	stats := targetURLFixStats{}
	var lastID int64

	for {
		var rows []commitStatusTargetURLRow
		if err := db.GetEngine(ctx).Table(table).
			Where("target_url LIKE ?", "%"+actionsRunPath+"%").
			And("id > ?", lastID).
			Asc("id").
			Limit(fixCommitStatusTargetURLBatchSize).
			Find(&rows); err != nil {
			return stats, fmt.Errorf("query %s: %w", table, err)
		}
		if len(rows) == 0 {
			break
		}

		for _, row := range rows {
			lastID = row.ID
			stats.scanned++
			stats.matched++

			newURL, err := convertCommitStatusTargetURL(ctx, row, repoLinkCache, runByIndexCache, jobsByRunIDCache)
			if err != nil {
				stats.unfixable++
				continue
			}

			stats.fixable++
			if !autofix {
				continue
			}

			if _, err := db.GetEngine(ctx).Table(table).ID(row.ID).Cols("target_url").Update(&commitStatusTargetURLRow{TargetURL: newURL}); err != nil {
				return stats, fmt.Errorf("update %s id=%d: %w", table, row.ID, err)
			}
			stats.updated++
		}
	}

	if stats.matched == 0 {
		logger.Info("%s: found no Actions target URLs", table)
		return stats, nil
	}
	if autofix {
		logger.Info("%s: scanned %d rows, updated %d rows, %d unfixable.", table, stats.scanned, stats.updated, stats.unfixable)
	} else {
		logger.Warn("%s: scanned %d rows, found %d fixable rows, %d unfixable.", table, stats.scanned, stats.fixable, stats.unfixable)
	}
	return stats, nil
}

func convertCommitStatusTargetURL(
	ctx context.Context,
	row commitStatusTargetURLRow,
	repoLinkCache map[int64]string,
	runByIndexCache map[int64]map[int64]*actions_model.ActionRun,
	jobsByRunIDCache map[int64][]int64,
) (string, error) {
	repoLink, err := getRepoLinkCached(ctx, repoLinkCache, row.RepoID)
	if err != nil {
		return "", fmt.Errorf("get repo link: %w", err)
	}
	if repoLink == "" {
		return "", fmt.Errorf("repo_id=%d not found", row.RepoID)
	}

	runNum, jobNum, ok := parseTargetURL(row.TargetURL, repoLink)
	if !ok {
		return "", errors.New("target_url does not match repo actions run URL format")
	}

	run, err := getRunByIndexCached(ctx, runByIndexCache, row.RepoID, runNum)
	if err != nil {
		return "", err
	}

	jobID, ok, err := getJobIDByIndexCached(ctx, jobsByRunIDCache, run.ID, jobNum)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("job not found for run_id=%d job_index=%d", run.ID, jobNum)
	}

	return fmt.Sprintf("%s%s%d/jobs/%d", repoLink, actionsRunPath, run.ID, jobID), nil
}

func getRepoLinkCached(ctx context.Context, cache map[int64]string, repoID int64) (string, error) {
	if link, ok := cache[repoID]; ok {
		return link, nil
	}

	repo := &repo_model.Repository{}
	has, err := db.GetEngine(ctx).Table("repository").Where("id=?", repoID).Cols("owner_name", "name").Get(repo)
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

func getRunByIndexCached(ctx context.Context, cache map[int64]map[int64]*actions_model.ActionRun, repoID, runIndex int64) (*actions_model.ActionRun, error) {
	if repoCache, ok := cache[repoID]; ok {
		if run, ok := repoCache[runIndex]; ok {
			if run == nil {
				return nil, fmt.Errorf("run not found for repo_id=%d run_index=%d", repoID, runIndex)
			}
			return run, nil
		}
	}

	var run actions_model.ActionRun
	has, err := db.GetEngine(ctx).Table("action_run").Where("repo_id=? AND `index`=?", repoID, runIndex).Get(&run)
	if err != nil {
		return nil, err
	}
	if cache[repoID] == nil {
		cache[repoID] = make(map[int64]*actions_model.ActionRun)
	}
	if !has {
		cache[repoID][runIndex] = nil
		return nil, fmt.Errorf("run not found for repo_id=%d run_index=%d", repoID, runIndex)
	}

	cache[repoID][runIndex] = &run
	return &run, nil
}

func getJobIDByIndexCached(ctx context.Context, cache map[int64][]int64, runID, jobIndex int64) (int64, bool, error) {
	jobIDs, ok := cache[runID]
	if !ok {
		var jobs []actions_model.ActionRunJob
		if err := db.GetEngine(ctx).Table("action_run_job").Where("run_id=?", runID).Asc("id").Cols("id").Find(&jobs); err != nil {
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
	if len(parts) != 3 || parts[1] != "jobs" {
		return 0, 0, false
	}

	runNum, err1 := strconv.ParseInt(parts[0], 10, 64)
	jobNum, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return runNum, jobNum, true
}
