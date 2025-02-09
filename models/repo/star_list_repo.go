package repo

import (
	"context"

	"code.gitea.io/gitea/models/db"
)

type StarListRepo struct {
	UID        int64 `xorm:"UNIQUE(s)"`
	StarListID int64 `xorm:"UNIQUE(s)"`
	RepoID     int64 `xorm:"UNIQUE(s)"`
}

func init() {
	db.RegisterModel(new(StarListRepo))
}

func StarLists(ctx context.Context, uid, repoID int64, ids []int64) error {
	starListRepos := make([]*StarListRepo, 0, len(ids))
	for _, id := range ids {
		starListRepos = append(starListRepos, &StarListRepo{
			UID:        uid,
			StarListID: id,
			RepoID:     repoID,
		})
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	_, err = db.GetEngine(ctx).Insert(&starListRepos)
	if err != nil {
		return err
	}

	_, err = db.GetEngine(ctx).Where("uid = ? AND repo_id = ? AND star_list_id NOT IN (?)", uid, repoID, ids).Delete(new(StarListRepo))
	if err != nil {
		return err
	}

	return committer.Commit()
}

func UnStarLists(ctx context.Context, uid, repoID int64, ids []int64) error {
	_, err := db.GetEngine(ctx).Where("uid = ? AND repo_id = ? AND star_list_id NOT IN (?)", uid, repoID, ids).Delete(new(StarListRepo))
	if err != nil {
		return err
	}
	return nil
}
