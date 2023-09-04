package issues

import (
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"context"
	"xorm.io/builder"
)

// CheckIssueSubscriber check if a user is subscribing an issue
// it takes participants and repo watch into account
func CheckIssueSubscriber(user *user_model.User, issue *Issue) (bool, error) {
	iw, exist, err := GetIssueWatch(db.DefaultContext, user.ID, issue.ID)
	if err != nil {
		return false, err
	}
	if exist {
		return iw.IsWatching, nil
	}
	w, err := repo_model.GetWatch(db.DefaultContext, user.ID, issue.RepoID)
	if err != nil {
		return false, err
	}
	return repo_model.IsWatchMode(w.Mode) || IsUserParticipantsOfIssue(user, issue), nil
}

// GetIssueSubscribers returns subscribers of a given issue
func GetIssueSubscribers(ctx context.Context, issue *Issue, listOptions db.ListOptions) (user_model.UserList, error) {
	subscribeUserIds := builder.Select("`issue_watch`.user_id").
		From("issue_watch").
		Where(builder.Eq{"`issue_watch`.issue_id": issue.ID, "`issue_watch`.is_watching": true})

	unsubscribeUserIds := builder.Select("`issue_watch`.user_id").
		From("issue_watch").
		Where(builder.Eq{"`issue_watch`.issue_id": issue.ID, "`issue_watch`.is_watching": false})

	participantsUserIds := builder.Select("`comment`.poster_id").
		From("comment").
		Where(builder.Eq{"`comment`.issue_id": issue.ID}).
		And(builder.In("`comment`.type", CommentTypeComment, CommentTypeCode, CommentTypeReview))

	repoSubscribeUserIds := builder.Select("`watch`.user_id").
		From("watch").
		Where(builder.Eq{"`watch`.repo_id": issue.RepoID}).
		And(builder.NotIn("`watch`.mode", repo_model.WatchModeDont, repo_model.WatchModeNone))

	sess := db.GetEngine(ctx).Table("user").
		Where(builder.Or(
			builder.In("id", subscribeUserIds),
			builder.In("id", repoSubscribeUserIds),
			builder.In("id", participantsUserIds),
			builder.In("id", issue.PosterID),
		).
			And(builder.NotIn("id", unsubscribeUserIds)).
			And(builder.Eq{"`user`.is_active": true, "`user`.prohibit_login": false}))

	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)
		users := make(user_model.UserList, 0, listOptions.PageSize)
		return users, sess.Find(&users)
	}
	users := make(user_model.UserList, 0, 8)
	return users, sess.Find(&users)
}

// CountIssueSubscribers count subscribers of a given issue
func CountIssueSubscribers(ctx context.Context, issue *Issue) (int64, error) {
	subscribeUserIds := builder.Select("`issue_watch`.user_id").
		From("issue_watch").
		Where(builder.Eq{"`issue_watch`.issue_id": issue.ID, "`issue_watch`.is_watching": true})

	unsubscribeUserIds := builder.Select("`issue_watch`.user_id").
		From("issue_watch").
		Where(builder.Eq{"`issue_watch`.issue_id": issue.ID, "`issue_watch`.is_watching": false})

	participantsUserIds := builder.Select("`comment`.poster_id").
		From("comment").
		Where(builder.Eq{"`comment`.issue_id": issue.ID}).
		And(builder.In("`comment`.type", CommentTypeComment, CommentTypeCode, CommentTypeReview))

	repoSubscribeUserIds := builder.Select("`watch`.user_id").
		From("watch").
		Where(builder.Eq{"`watch`.repo_id": issue.RepoID}).
		And(builder.NotIn("`watch`.mode", repo_model.WatchModeDont, repo_model.WatchModeNone))

	sess := db.GetEngine(ctx).Table("user").
		Where(builder.Or(
			builder.In("id", subscribeUserIds),
			builder.In("id", repoSubscribeUserIds),
			builder.In("id", participantsUserIds),
			builder.In("id", issue.PosterID),
		).
			And(builder.NotIn("id", unsubscribeUserIds)).
			And(builder.Eq{"`user`.is_active": true, "`user`.prohibit_login": false}))

	return sess.Count(new(user_model.User))
}
