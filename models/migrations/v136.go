// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	pull_service "code.gitea.io/gitea/services/pull"

	"xorm.io/xorm"
)

func addCommitDivergenceToPulls(x *xorm.Engine) error {

	if err := x.Sync2(new(models.PullRequest)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	var last int
	batchSize := setting.Database.IterateBufferSize
	sess := x.NewSession()
	defer sess.Close()
	for {
		if err := sess.Begin(); err != nil {
			return err
		}
		var results = make([]*models.PullRequest, 0, batchSize)
		err := sess.Where("has_merged = ?", false).OrderBy("id").Limit(batchSize, last).Find(&results)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			break
		}
		last += len(results)

		for _, pr := range results {
			divergence, err := pull_service.GetDiverging(pr)
			if err != nil {
				if err = pr.LoadIssue(); err != nil {
					return fmt.Errorf("pr.LoadIssue()[%d]: %v", pr.ID, err)
				}
				if !pr.Issue.IsClosed {
					return fmt.Errorf("GetDiverging: %v", err)
				}
				log.Warn("Could not recalculate Divergence for pull: %d", pr.ID)
				pr.CommitsAhead = 0
				pr.CommitsBehind = 0
			}
			if divergence != nil {
				pr.CommitsAhead = divergence.Ahead
				pr.CommitsBehind = divergence.Behind
			}
			if _, err = sess.ID(pr.ID).Cols("commits_ahead", "commits_behind").Update(pr); err != nil {
				return fmt.Errorf("Update Cols: %v", err)
			}
		}

		if err := sess.Commit(); err != nil {
			return err
		}
	}
	return nil
}
