package models

import (
	"context"
	"strings"
	"time"

	"code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/user"

	"xorm.io/builder"
)

// GetRecentlyPushedBranches returns all actions where a user recently pushed but no PRs are created yet.
func GetRecentlyPushedBranches(ctx context.Context, u *user.User) (actions []*activities.Action, err error) {

	limit := time.Now().Add(-24 * time.Hour).Unix()

	err = db.GetEngine(ctx).
		Select("action.ref_name, action.repo_id, replace(action.ref_name, 'refs/heads/', '') AS clean_ref_name").
		Join("LEFT", "pull_request", "pull_request.head_branch = clean_ref_name").
		Join("LEFT", "issue", "pull_request.issue_id = issue.id").
		Join("LEFT", "repository", "action.repo_id = repository.id").
		Where(builder.And(
			builder.Eq{"action.op_type": activities.ActionCommitRepo},
			builder.Eq{"action.act_user_id": u.ID},
			builder.Or(
				builder.Expr("repository.default_branch != clean_ref_name"),
				builder.Eq{"repository.is_fork": true},
			),
			builder.Or(
				builder.IsNull{"pull_request.id"},
				builder.And(
					builder.Eq{"pull_request.has_merged": false},
					builder.Eq{"issue.is_closed": true},
					builder.Gt{"action.created_unix": "issue.closed_unix"},
				),
			),
			builder.Gte{"action.created_unix": limit},
		)).
		Limit(3).
		GroupBy("action.ref_name, action.repo_id, clean_ref_name").
		Desc("action.id").
		Find(&actions)
	if err != nil {
		return nil, err
	}

	repoIDs := []int64{}
	for _, a := range actions {
		repoIDs = append(repoIDs, a.RepoID)
	}

	repos := make(map[int64]*repo.Repository, len(repoIDs))
	err = db.GetEngine(ctx).In("id", repoIDs).Find(&repos)
	if err != nil {
		return nil, err
	}

	owners := make(map[int64]*user.User)
	err = db.GetEngine(ctx).
		In("repository.id", repoIDs).
		Join("LEFT", "repository", "repository.owner_id = u.id").
		Find(&owners)
	if err != nil {
		return nil, err
	}

	for _, a := range actions {
		a.Repo = repos[a.RepoID]
		a.Repo.Owner = owners[a.Repo.OwnerID]
		a.RefName = strings.Replace(a.RefName, "refs/heads/", "", 1)
	}

	return
}
