
package repo

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitdiff"
	"code.gitea.io/gitea/modules/timeutil"
)

// CommitComment represents a comment on a commit
type CommitComment struct {
	ID          int64 `xorm:"pk autoincr"`
	RepoID      int64 `xorm:"INDEX"`
	CommitSHA   string
	PosterID    int64
	Poster      *user_model.User `xorm:"-"`
	Line        int64
	TreePath    string
	Content     string `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// CreateCommitComment creates a new commit comment
func CreateCommitComment(ctx context.Context, opts *CreateCommitCommentOptions) (*CommitComment, error) {
	comment := &CommitComment{
		RepoID:    opts.RepoID,
		CommitSHA: opts.CommitSHA,
		PosterID:  opts.PosterID,
		Line:      opts.Line,
		TreePath:  opts.TreePath,
		Content:   opts.Content,
	}

	if _, err := db.GetEngine(ctx).Insert(comment); err != nil {
		return nil, err
	}

	return comment, nil
}

// CreateCommitCommentOptions defines options for creating a commit comment
type CreateCommitCommentOptions struct {
	RepoID    int64
	CommitSHA string
	PosterID  int64
	Line      int64
	TreePath  string
	Content   string
}

// GetCommitComments returns all comments for a commit
func GetCommitComments(ctx context.Context, repoID int64, commitSHA string) ([]*CommitComment, error) {
	comments := make([]*CommitComment, 0, 10)
	if err := db.GetEngine(ctx).
		Where("repo_id = ?", repoID).
		And("commit_sha = ?", commitSHA).
		OrderBy("created_unix ASC").
		Find(&comments); err != nil {
		return nil, fmt.Errorf("get commit comments: %v", err)
	}

	return comments, nil
}

// GetCommitCommentByID returns a commit comment by ID
func GetCommitCommentByID(ctx context.Context, id int64) (*CommitComment, error) {
	comment := new(CommitComment)
	has, err := db.GetEngine(ctx).ID(id).Get(comment)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("commit comment does not exist [id: %d]", id)
	}
	return comment, nil
}

// LoadPoster loads poster for commit comment
func (c *CommitComment) LoadPoster(ctx context.Context) error {
	if c.Poster != nil {
		return nil
	}
	var err error
	c.Poster, err = user_model.GetUserByID(ctx, c.PosterID)
	return err
}

// LoadCommentsForDiffLines loads comments for diff lines
func LoadCommentsForDiffLines(ctx context.Context, repoID int64, commitSHA string, diffLines []*gitdiff.DiffLine) error {
	// Get all comments for this commit
	comments, err := GetCommitComments(ctx, repoID, commitSHA)
	if err != nil {
		return err
	}

	// Map comments by line and path
	commentMap := make(map[string][]*CommitComment)
	for _, comment := range comments {
		key := fmt.Sprintf("%d-%s", comment.Line, comment.TreePath)
		commentMap[key] = append(commentMap[key], comment)
	}

	// Attach comments to diff lines
	for _, line := range diffLines {
		var key string
		if line.RightIdx > 0 {
			key = fmt.Sprintf("%d-%s", line.RightIdx, line.FileName)
		} else if line.LeftIdx > 0 {
			key = fmt.Sprintf("%d-%s", line.LeftIdx, line.FileName)
		}

		if comments, ok := commentMap[key]; ok {
			for _, comment := range comments {
				if err := comment.LoadPoster(ctx); err != nil {
					return err
				}
			}
			line.Comments = append(line.Comments, convertCommitCommentsToIssueComments(comments)...)
		}
	}

	return nil
}

// convertCommitCommentsToIssueComments converts commit comments to issue comments for display
func convertCommitCommentsToIssueComments(commitComments []*CommitComment) []*issues_model.Comment {
	issueComments := make([]*issues_model.Comment, len(commitComments))
	for i, cc := range commitComments {
		issueComments[i] = &issues_model.Comment{
			ID:          cc.ID,
			PosterID:    cc.PosterID,
			Poster:      cc.Poster,
			IssueID:     0, // Not linked to an issue
			Content:     cc.Content,
			CreatedUnix: cc.CreatedUnix,
			UpdatedUnix: cc.UpdatedUnix,
			Type:        issues_model.CommentTypeCommit,
		}
	}
	return issueComments
}
