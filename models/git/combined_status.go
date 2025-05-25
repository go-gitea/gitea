// Copyright 2024 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/commitstatus"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/translation"

	"xorm.io/builder"
)

// CombinedStatus holds the latest combined Status of a single Commit
type CombinedStatus struct {
	ID        int64                            `xorm:"pk autoincr"`
	RepoID    int64                            `xorm:"INDEX UNIQUE(repo_id_sha)"`
	Repo      *repo_model.Repository           `xorm:"-"`
	SHA       string                           `xorm:"VARCHAR(64) NOT NULL INDEX UNIQUE(repo_id_sha)"`
	State     commitstatus.CombinedStatusState `xorm:"VARCHAR(7) NOT NULL"`
	TargetURL string                           `xorm:"TEXT"`
}

func (CombinedStatus) TableName() string {
	return "commit_status_summary" // legacy name for compatibility
}

func init() {
	db.RegisterModel(new(CombinedStatus))
}

func (status *CombinedStatus) loadRepository(ctx context.Context) error {
	if status.RepoID == 0 || status.Repo != nil {
		return nil
	}

	repo, err := repo_model.GetRepositoryByID(ctx, status.RepoID)
	if err != nil {
		return err
	}
	status.Repo = repo

	return nil
}

// LocaleString returns the locale string name of the Status
func (status *CombinedStatus) LocaleString(lang translation.Locale) string {
	return lang.TrString("repo.commitstatus." + status.State.String())
}

// HideActionsURL set `TargetURL` to an empty string if the status comes from Gitea Actions
func (status *CombinedStatus) HideActionsURL(ctx context.Context) {
	if status.RepoID == 0 {
		return
	}

	if err := status.loadRepository(ctx); err != nil {
		log.Error("loadRepository: %v", err)
		return
	}

	prefix := status.Repo.Link() + "/actions"
	if strings.HasPrefix(status.TargetURL, prefix) {
		status.TargetURL = ""
	}
}

type RepoSHA struct {
	RepoID int64
	SHA    string
}

func GetLatestCombinedStatusForRepoAndSHAs(ctx context.Context, repoSHAs []RepoSHA) ([]*CombinedStatus, error) {
	cond := builder.NewCond()
	for _, rs := range repoSHAs {
		cond = cond.Or(builder.Eq{"repo_id": rs.RepoID, "sha": rs.SHA})
	}

	var summaries []*CombinedStatus
	if err := db.GetEngine(ctx).Where(cond).Find(&summaries); err != nil {
		return nil, err
	}
	return summaries, nil
}

func InsertOrUpdateCombinedStatus(ctx context.Context, repoID int64, sha string) error {
	commitStatuses, _, err := GetLatestCommitStatus(ctx, repoID, sha, db.ListOptionsAll)
	if err != nil {
		return err
	}
	combinedStatus := CalcCombinedStatus(commitStatuses)

	// mysql will return 0 when update a record which state hasn't been changed which behaviour is different from other database,
	// so we need to use insert in on duplicate
	if setting.Database.Type.IsMySQL() {
		_, err := db.GetEngine(ctx).Exec("INSERT INTO commit_status_summary (repo_id,sha,state,target_url) VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE state=?",
			repoID, sha, combinedStatus.State, combinedStatus.TargetURL, combinedStatus.State)
		return err
	}

	if cnt, err := db.GetEngine(ctx).Where("repo_id=? AND sha=?", repoID, sha).
		Cols("state, target_url").
		Update(combinedStatus); err != nil {
		return err
	} else if cnt == 0 {
		_, err = db.GetEngine(ctx).Insert(combinedStatus)
		return err
	}
	return nil
}

func CombinedStatusesHideActionsURL(ctx context.Context, statuses []*CombinedStatus) {
	idToRepos := make(map[int64]*repo_model.Repository)
	for _, status := range statuses {
		if status == nil {
			continue
		}

		if status.Repo == nil {
			status.Repo = idToRepos[status.RepoID]
		}
		status.HideActionsURL(ctx)
		idToRepos[status.RepoID] = status.Repo
	}
}
