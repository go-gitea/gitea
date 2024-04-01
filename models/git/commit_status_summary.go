// Copyright 2017 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	"code.gitea.io/gitea/models/db"
	api "code.gitea.io/gitea/modules/structs"
	"xorm.io/builder"
)

// CommitStatusSummary holds the latest commit Status of a single Commit
type CommitStatusSummary struct {
	ID     int64                 `xorm:"pk autoincr"`
	RepoID int64                 `xorm:"INDEX UNIQUE(repo_sha_index)"`
	SHA    string                `xorm:"VARCHAR(64) NOT NULL INDEX UNIQUE(repo_sha_index)"`
	State  api.CommitStatusState `xorm:"VARCHAR(7) NOT NULL"`
}

func init() {
	db.RegisterModel(new(CommitStatusSummary))
}

type RepoSha struct {
	RepoID int64
	SHA    string
}

func GetLatestCommitStatusForRepoAndSHAs(ctx context.Context, repoShas []RepoSha) ([]*CommitStatus, error) {
	cond := builder.NewCond()
	for _, rs := range repoShas {
		cond = cond.Or(builder.Eq{"repo_id": rs.RepoID, "sha": rs.SHA})
	}

	var summaries []CommitStatusSummary
	if err := db.GetEngine(ctx).Where(cond).Find(&summaries); err != nil {
		return nil, err
	}

	commitStatuses := make([]*CommitStatus, 0, len(repoShas))
	for _, summary := range summaries {
		commitStatuses = append(commitStatuses, &CommitStatus{
			RepoID: summary.RepoID,
			SHA:    summary.SHA,
			State:  summary.State,
		})
	}
	return commitStatuses, nil
}

func UpdateCommitStatusSummary(ctx context.Context, repoID int64, sha string) error {
	commitStatuses, _, err := GetLatestCommitStatus(ctx, repoID, sha, db.ListOptionsAll)
	if err != nil {
		return err
	}
	state := CalcCommitStatus(commitStatuses)
	if cnt, err := db.GetEngine(ctx).Where("repo_id=? AND sha=?", repoID, sha).
		Cols("state").
		Update(&CommitStatusSummary{
			State: state.State,
		}); err != nil {
		return err
	} else if cnt == 0 {
		_, err = db.GetEngine(ctx).Insert(&CommitStatusSummary{
			RepoID: repoID,
			SHA:    sha,
			State:  state.State,
		})
		return err
	}
	return nil
}
