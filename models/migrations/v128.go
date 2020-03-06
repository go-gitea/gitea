// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func fixMergeBase(x *xorm.Engine) error {
	type Repository struct {
		ID        int64 `xorm:"pk autoincr"`
		OwnerID   int64 `xorm:"UNIQUE(s) index"`
		OwnerName string
		LowerName string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Name      string `xorm:"INDEX NOT NULL"`
	}

	type PullRequest struct {
		ID         int64 `xorm:"pk autoincr"`
		Index      int64
		HeadRepoID int64 `xorm:"INDEX"`
		BaseRepoID int64 `xorm:"INDEX"`
		HeadBranch string
		BaseBranch string
		MergeBase  string `xorm:"VARCHAR(40)"`

		HasMerged      bool   `xorm:"INDEX"`
		MergedCommitID string `xorm:"VARCHAR(40)"`
	}

	var limit = setting.Database.IterateBufferSize
	if limit <= 0 {
		limit = 50
	}

	i := 0
	for {
		prs := make([]PullRequest, 0, 50)
		if err := x.Limit(limit, i).Asc("id").Find(&prs); err != nil {
			return fmt.Errorf("Find: %v", err)
		}
		if len(prs) == 0 {
			break
		}

		i += 50
		for _, pr := range prs {
			baseRepo := &Repository{ID: pr.BaseRepoID}
			has, err := x.Table("repository").Get(baseRepo)
			if err != nil {
				return fmt.Errorf("Unable to get base repo %d %v", pr.BaseRepoID, err)
			}
			if !has {
				log.Error("Missing base repo with id %d for PR ID %d", pr.BaseRepoID, pr.ID)
				continue
			}
			userPath := filepath.Join(setting.RepoRootPath, strings.ToLower(baseRepo.OwnerName))
			repoPath := filepath.Join(userPath, strings.ToLower(baseRepo.Name)+".git")

			gitRefName := fmt.Sprintf("refs/pull/%d/head", pr.Index)

			if !pr.HasMerged {
				var err error
				pr.MergeBase, err = git.NewCommand("merge-base", "--", pr.BaseBranch, gitRefName).RunInDir(repoPath)
				if err != nil {
					var err2 error
					pr.MergeBase, err2 = git.NewCommand("rev-parse", git.BranchPrefix+pr.BaseBranch).RunInDir(repoPath)
					if err2 != nil {
						log.Error("Unable to get merge base for PR ID %d, Index %d in %s/%s. Error: %v & %v", pr.ID, pr.Index, baseRepo.OwnerName, baseRepo.Name, err, err2)
						continue
					}
				}
			} else {
				var err error
				pr.MergeBase, err = git.NewCommand("merge-base", "--", pr.MergedCommitID+"^", gitRefName).RunInDir(repoPath)
				if err != nil {
					log.Error("Unable to get merge base for merged PR ID %d, Index %d in %s/%s. Error: %", pr.ID, pr.Index, baseRepo.OwnerName, baseRepo.Name, err)
					continue
				}
			}
			pr.MergeBase = strings.TrimSpace(pr.MergeBase)
			x.ID(pr.ID).Cols("merge_base").Update(pr)
		}
	}

	return nil
}
