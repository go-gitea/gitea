// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"
	"strconv"

	_ "image/jpeg" // Needed for jpeg support

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"

	"xorm.io/builder"
)

// Init initialize model
func Init(ctx context.Context) error {
	return unit.LoadUnitConfig()
}

type repoChecker struct {
	querySQL   func(ctx context.Context) ([]int64, error)
	correctSQL func(ctx context.Context, id int64) error
	desc       string
}

func repoStatsCheck(ctx context.Context, checker *repoChecker) {
	results, err := checker.querySQL(ctx)
	if err != nil {
		log.Error("Select %s: %v", checker.desc, err)
		return
	}
	for _, id := range results {
		select {
		case <-ctx.Done():
			log.Warn("CheckRepoStats: Cancelled before checking %s for with id=%d", checker.desc, id)
			return
		default:
		}
		log.Trace("Updating %s: %d", checker.desc, id)
		err = checker.correctSQL(ctx, id)
		if err != nil {
			log.Error("Update %s[%d]: %v", checker.desc, id, err)
		}
	}
}

func StatsCorrectSQL(ctx context.Context, sql any, ids ...any) error {
	args := []any{sql}
	args = append(args, ids...)
	_, err := db.GetEngine(ctx).Exec(args...)
	return err
}

func repoStatsCorrectNumWatches(ctx context.Context, id int64) error {
	return StatsCorrectSQL(ctx, "UPDATE `repository` SET num_watches=(SELECT COUNT(*) FROM `watch` WHERE repo_id=? AND mode<>2) WHERE id=?", id, id)
}

func repoStatsCorrectNumStars(ctx context.Context, id int64) error {
	return StatsCorrectSQL(ctx, "UPDATE `repository` SET num_stars=(SELECT COUNT(*) FROM `star` WHERE repo_id=?) WHERE id=?", id, id)
}

func labelStatsCorrectNumIssues(ctx context.Context, id int64) error {
	return StatsCorrectSQL(ctx, "UPDATE `label` SET num_issues=(SELECT COUNT(*) FROM `issue_label` WHERE label_id=?) WHERE id=?", id, id)
}

func labelStatsCorrectNumIssuesRepo(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `label` SET num_issues=(SELECT COUNT(*) FROM `issue_label` WHERE label_id=id) WHERE repo_id=?", id)
	return err
}

func labelStatsCorrectNumClosedIssues(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `label` SET num_closed_issues=(SELECT COUNT(*) FROM `issue_label`,`issue` WHERE `issue_label`.label_id=`label`.id AND `issue_label`.issue_id=`issue`.id AND `issue`.is_closed=?) WHERE `label`.id=?", true, id)
	return err
}

func labelStatsCorrectNumClosedIssuesRepo(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `label` SET num_closed_issues=(SELECT COUNT(*) FROM `issue_label`,`issue` WHERE `issue_label`.label_id=`label`.id AND `issue_label`.issue_id=`issue`.id AND `issue`.is_closed=?) WHERE `label`.repo_id=?", true, id)
	return err
}

var milestoneStatsQueryNumIssues = "SELECT `milestone`.id FROM `milestone` WHERE `milestone`.num_closed_issues!=(SELECT COUNT(*) FROM `issue` WHERE `issue`.milestone_id=`milestone`.id AND `issue`.is_closed=?) OR `milestone`.num_issues!=(SELECT COUNT(*) FROM `issue` WHERE `issue`.milestone_id=`milestone`.id)"

func milestoneStatsCorrectNumIssuesRepo(ctx context.Context, id int64) error {
	e := db.GetEngine(ctx)
	results, err := e.Query(milestoneStatsQueryNumIssues+" AND `milestone`.repo_id = ?", true, id)
	if err != nil {
		return err
	}
	for _, result := range results {
		id, _ := strconv.ParseInt(string(result["id"]), 10, 64)
		err = issues_model.UpdateMilestoneCounters(ctx, id)
		if err != nil {
			return err
		}
	}
	return nil
}

func userStatsCorrectNumRepos(ctx context.Context, id int64) error {
	return StatsCorrectSQL(ctx, "UPDATE `user` SET num_repos=(SELECT COUNT(*) FROM `repository` WHERE owner_id=?) WHERE id=?", id, id)
}

func repoStatsCorrectIssueNumComments(ctx context.Context, id int64) error {
	return StatsCorrectSQL(ctx, issues_model.UpdateIssueNumCommentsBuilder(id))
}

func repoStatsCorrectNumIssues(ctx context.Context, id int64) error {
	return repo_model.UpdateRepoIssueNumbers(ctx, id, false, false)
}

func repoStatsCorrectNumPulls(ctx context.Context, id int64) error {
	return repo_model.UpdateRepoIssueNumbers(ctx, id, true, false)
}

func repoStatsCorrectNumClosedIssues(ctx context.Context, id int64) error {
	return repo_model.UpdateRepoIssueNumbers(ctx, id, false, true)
}

func repoStatsCorrectNumClosedPulls(ctx context.Context, id int64) error {
	return repo_model.UpdateRepoIssueNumbers(ctx, id, true, true)
}

// statsQuery returns a function that queries the database for a list of IDs
// sql could be a string or a *builder.Builder
func statsQuery(sql any, args ...any) func(context.Context) ([]int64, error) {
	return func(ctx context.Context) ([]int64, error) {
		var ids []int64
		return ids, db.GetEngine(ctx).SQL(sql, args...).Find(&ids)
	}
}

// CheckRepoStats checks the repository stats
func CheckRepoStats(ctx context.Context) error {
	log.Trace("Doing: CheckRepoStats")

	checkers := []*repoChecker{
		// Repository.NumWatches
		{
			statsQuery("SELECT repo.id FROM `repository` repo WHERE repo.num_watches!=(SELECT COUNT(*) FROM `watch` WHERE repo_id=repo.id AND mode<>2)"),
			repoStatsCorrectNumWatches,
			"repository count 'num_watches'",
		},
		// Repository.NumStars
		{
			statsQuery("SELECT repo.id FROM `repository` repo WHERE repo.num_stars!=(SELECT COUNT(*) FROM `star` WHERE repo_id=repo.id)"),
			repoStatsCorrectNumStars,
			"repository count 'num_stars'",
		},
		// Repository.NumIssues
		{
			statsQuery("SELECT repo.id FROM `repository` repo WHERE repo.num_issues!=(SELECT COUNT(*) FROM `issue` WHERE repo_id=repo.id AND is_pull=?)", false),
			repoStatsCorrectNumIssues,
			"repository count 'num_issues'",
		},
		// Repository.NumClosedIssues
		{
			statsQuery("SELECT repo.id FROM `repository` repo WHERE repo.num_closed_issues!=(SELECT COUNT(*) FROM `issue` WHERE repo_id=repo.id AND is_closed=? AND is_pull=?)", true, false),
			repoStatsCorrectNumClosedIssues,
			"repository count 'num_closed_issues'",
		},
		// Repository.NumPulls
		{
			statsQuery("SELECT repo.id FROM `repository` repo WHERE repo.num_pulls!=(SELECT COUNT(*) FROM `issue` WHERE repo_id=repo.id AND is_pull=?)", true),
			repoStatsCorrectNumPulls,
			"repository count 'num_pulls'",
		},
		// Repository.NumClosedPulls
		{
			statsQuery("SELECT repo.id FROM `repository` repo WHERE repo.num_closed_pulls!=(SELECT COUNT(*) FROM `issue` WHERE repo_id=repo.id AND is_closed=? AND is_pull=?)", true, true),
			repoStatsCorrectNumClosedPulls,
			"repository count 'num_closed_pulls'",
		},
		// Label.NumIssues
		{
			statsQuery("SELECT label.id FROM `label` WHERE label.num_issues!=(SELECT COUNT(*) FROM `issue_label` WHERE label_id=label.id)"),
			labelStatsCorrectNumIssues,
			"label count 'num_issues'",
		},
		// Label.NumClosedIssues
		{
			statsQuery("SELECT `label`.id FROM `label` WHERE `label`.num_closed_issues!=(SELECT COUNT(*) FROM `issue_label`,`issue` WHERE `issue_label`.label_id=`label`.id AND `issue_label`.issue_id=`issue`.id AND `issue`.is_closed=?)", true),
			labelStatsCorrectNumClosedIssues,
			"label count 'num_closed_issues'",
		},
		// Milestone.Num{,Closed}Issues
		{
			statsQuery(milestoneStatsQueryNumIssues, true),
			issues_model.UpdateMilestoneCounters,
			"milestone count 'num_closed_issues' and 'num_issues'",
		},
		// User.NumRepos
		{
			statsQuery("SELECT `user`.id FROM `user` WHERE `user`.num_repos!=(SELECT COUNT(*) FROM `repository` WHERE owner_id=`user`.id)"),
			userStatsCorrectNumRepos,
			"user count 'num_repos'",
		},
		// Issue.NumComments
		{
			statsQuery(builder.Select("`issue`.id").From("`issue`").Where(
				builder.Neq{
					"`issue`.num_comments": builder.Select("COUNT(*)").From("`comment`").Where(
						builder.Expr("issue_id = `issue`.id").And(
							builder.In("type", issues_model.ConversationCountedCommentType()),
						),
					),
				},
			),
			),
			repoStatsCorrectIssueNumComments,
			"issue count 'num_comments'",
		},
	}
	for _, checker := range checkers {
		select {
		case <-ctx.Done():
			log.Warn("CheckRepoStats: Cancelled before %s", checker.desc)
			return db.ErrCancelledf("before checking %s", checker.desc)
		default:
			repoStatsCheck(ctx, checker)
		}
	}

	// FIXME: use checker when stop supporting old fork repo format.
	// ***** START: Repository.NumForks *****
	e := db.GetEngine(ctx)
	results, err := e.Query("SELECT repo.id FROM `repository` repo WHERE repo.num_forks!=(SELECT COUNT(*) FROM `repository` WHERE fork_id=repo.id)")
	if err != nil {
		log.Error("Select repository count 'num_forks': %v", err)
	} else {
		for _, result := range results {
			id, _ := strconv.ParseInt(string(result["id"]), 10, 64)
			select {
			case <-ctx.Done():
				log.Warn("CheckRepoStats: Cancelled")
				return db.ErrCancelledf("during repository count 'num_fork' for repo ID %d", id)
			default:
			}
			log.Trace("Updating repository count 'num_forks': %d", id)

			repo, err := repo_model.GetRepositoryByID(ctx, id)
			if err != nil {
				log.Error("repo_model.GetRepositoryByID[%d]: %v", id, err)
				continue
			}

			_, err = e.SQL("SELECT COUNT(*) FROM `repository` WHERE fork_id=?", repo.ID).Get(&repo.NumForks)
			if err != nil {
				log.Error("Select count of forks[%d]: %v", repo.ID, err)
				continue
			}

			if _, err = e.ID(repo.ID).Cols("num_forks").Update(repo); err != nil {
				log.Error("UpdateRepository[%d]: %v", id, err)
				continue
			}
		}
	}
	// ***** END: Repository.NumForks *****
	return nil
}

func UpdateRepoStats(ctx context.Context, id int64) error {
	var err error

	for _, f := range []func(ctx context.Context, id int64) error{
		repoStatsCorrectNumWatches,
		repoStatsCorrectNumStars,
		repoStatsCorrectNumIssues,
		repoStatsCorrectNumPulls,
		repoStatsCorrectNumClosedIssues,
		repoStatsCorrectNumClosedPulls,
		labelStatsCorrectNumIssuesRepo,
		labelStatsCorrectNumClosedIssuesRepo,
		milestoneStatsCorrectNumIssuesRepo,
	} {
		err = f(ctx, id)
		if err != nil {
			return err
		}
	}
	return nil
}

func updateUserStarNumbers(ctx context.Context, users []user_model.User) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	for _, user := range users {
		if _, err = db.Exec(ctx, "UPDATE `user` SET num_stars=(SELECT COUNT(*) FROM `star` WHERE uid=?) WHERE id=?", user.ID, user.ID); err != nil {
			return err
		}
	}

	return committer.Commit()
}

// DoctorUserStarNum recalculate Stars number for all user
func DoctorUserStarNum(ctx context.Context) (err error) {
	const batchSize = 100

	for start := 0; ; start += batchSize {
		users := make([]user_model.User, 0, batchSize)
		if err = db.GetEngine(ctx).Limit(batchSize, start).Where("type = ?", 0).Cols("id").Find(&users); err != nil {
			return err
		}
		if len(users) == 0 {
			break
		}

		if err = updateUserStarNumbers(ctx, users); err != nil {
			return err
		}
	}

	log.Debug("recalculate Stars number for all user finished")

	return err
}
