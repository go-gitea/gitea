package automerge

import (
	"context"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	pull_service "code.gitea.io/gitea/services/pull"
)

// if schedule is false and no error, pull can be merged directly
func ScheduleAutoMerge(ctx context.Context, doer *user_model.User, pull *models.PullRequest, style repo_model.MergeStyle, message string) (scheduled bool, err error) {
	lastCommitStatus, err := pull_service.GetPullRequestCommitStatusState(ctx, pull)
	if err != nil {
		return false, err
	}

	// we dont need to schedule
	if lastCommitStatus.IsSuccess() {
		return true, nil
	}

	return true, models.ScheduleAutoMerge(doer, pull.ID, style, message)
}
