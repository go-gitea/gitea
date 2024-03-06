// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12 //nolint

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func AddCommitDivergenceToPulls(x *xorm.Engine) error {
	type Repository struct {
		ID        int64 `xorm:"pk autoincr"`
		OwnerID   int64 `xorm:"UNIQUE(s) index"`
		OwnerName string
		LowerName string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Name      string `xorm:"INDEX NOT NULL"`
	}

	type PullRequest struct {
		ID      int64 `xorm:"pk autoincr"`
		IssueID int64 `xorm:"INDEX"`
		Index   int64

		CommitsAhead  int
		CommitsBehind int

		BaseRepoID int64 `xorm:"INDEX"`
		BaseBranch string

		HasMerged      bool   `xorm:"INDEX"`
		MergedCommitID string `xorm:"VARCHAR(40)"`
	}

	if err := x.Sync(new(PullRequest)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}

	last := 0
	migrated := 0

	batchSize := setting.Database.IterateBufferSize
	sess := x.NewSession()
	defer sess.Close()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	count, err := sess.Where("has_merged = ?", false).Count(new(PullRequest))
	if err != nil {
		return err
	}
	log.Info("%d Unmerged Pull Request(s) to migrate ...", count)

	for {
		if err := sess.Begin(); err != nil {
			return err
		}
		results := make([]*PullRequest, 0, batchSize)
		err := sess.Where("has_merged = ?", false).OrderBy("id").Limit(batchSize, last).Find(&results)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			break
		}
		last += batchSize

		for _, pr := range results {
			baseRepo := &Repository{ID: pr.BaseRepoID}
			has, err := x.Table("repository").Get(baseRepo)
			if err != nil {
				return fmt.Errorf("Unable to get base repo %d %w", pr.BaseRepoID, err)
			}
			if !has {
				log.Error("Missing base repo with id %d for PR ID %d", pr.BaseRepoID, pr.ID)
				continue
			}
			userPath := filepath.Join(setting.RepoRootPath, strings.ToLower(baseRepo.OwnerName))
			repoPath := filepath.Join(userPath, strings.ToLower(baseRepo.Name)+".git")

			gitRefName := fmt.Sprintf("refs/pull/%d/head", pr.Index)

			divergence, err := git.GetDivergingCommits(graceful.GetManager().HammerContext(), repoPath, pr.BaseBranch, gitRefName)
			if err != nil {
				log.Warn("Could not recalculate Divergence for pull: %d", pr.ID)
				pr.CommitsAhead = 0
				pr.CommitsBehind = 0
			}
			pr.CommitsAhead = divergence.Ahead
			pr.CommitsBehind = divergence.Behind

			if _, err = sess.ID(pr.ID).Cols("commits_ahead", "commits_behind").Update(pr); err != nil {
				return fmt.Errorf("Update Cols: %w", err)
			}
			migrated++
		}

		if err := sess.Commit(); err != nil {
			return err
		}
		select {
		case <-ticker.C:
			log.Info(
				"%d/%d (%2.0f%%) Pull Request(s) migrated in %d batches. %d PRs Remaining ...",
				migrated,
				count,
				float64(migrated)/float64(count)*100,
				int(math.Ceil(float64(migrated)/float64(batchSize))),
				count-int64(migrated))
		default:
		}
	}
	log.Info("Completed migrating %d Pull Request(s) in: %d batches", count, int(math.Ceil(float64(migrated)/float64(batchSize))))
	return nil
}
