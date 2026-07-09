package repository

import (
	"context"

	"gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
)

// CreateCommitCodeComment creates a code comment on a commit
func CreateCommitCodeComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, commitID string, line int64, content, treePath string, attachments []string) (*issues.Comment, error) {
	commit, err := gitRepo.GetCommit(commitID)
	if err != nil {
		return nil, err
	}

	objectFormat, err := gitRepo.GetObjectFormat()
	if err != nil {
		return nil, err
	}
	parentCommitID := objectFormat.EmptyTree().String()

	if commit.ParentCount() > 0 {
		parentCommit, err := commit.Parent(0)
		if err == nil && parentCommit != nil {
			parentCommitID = parentCommit.ID.String()
		}
	}

	patch, err := git.GetFileDiffCutAroundLine(
		gitRepo, parentCommitID, commitID, treePath,
		int64((&issues.Comment{Line: line}).UnsignedLine()), line < 0, setting.UI.CodeCommentLines,
	)
	if err != nil {
		return nil, err
	}

	return issues.CreateComment(ctx, &issues.CreateCommentOptions{
		Type:        issues.CommentTypeCode,
		Doer:        doer,
		Repo:        repo,
		Content:     content,
		LineNum:     line,
		TreePath:    treePath,
		CommitSHA:   commitID,
		Patch:       patch,
		Attachments: attachments,
	})
}
