package issue

import (
	"errors"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

func CloseIssue(ctx *context.Context, issueID int64) error {

	issue, err := issues_model.GetIssueByID(ctx, issueID)
	if err != nil {
		return errors.New("failed getting issue")
	}

	if err := ChangeStatus(ctx, issue, ctx.Doer, "", true); err != nil {
		log.Error("ChangeStatus: %v", err)

		if issues_model.IsErrDependenciesLeft(err) {
			ctx.JSONError(ctx.Tr("repo.issues.dependency.issue_close_blocked"))
			return err
		}
	}

	return nil
}
