// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"strconv"

	"gitea.dev/models/db"
	"gitea.dev/models/renderhelper"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// CodeComments represents comments on code by using this structure: FILENAME -> LINE (+ == proposed; - == previous) -> COMMENTS
type CodeComments map[string]map[int64][]*Comment

// FetchCodeComments will return a 2d-map: ["Path"]["Line"] = Comments at line
func FetchCodeComments(ctx context.Context, issue *Issue, currentUser *user_model.User, showOutdatedComments bool) (CodeComments, error) {
	return fetchCodeCommentsByReview(ctx, issue, currentUser, nil, showOutdatedComments)
}

func fetchCodeCommentsByReview(ctx context.Context, issue *Issue, currentUser *user_model.User, review *Review, showOutdatedComments bool) (CodeComments, error) {
	pathToLineToComment := make(CodeComments)
	if review == nil {
		review = &Review{ID: 0}
	}
	opts := FindCommentsOptions{
		Type:     CommentTypeCode,
		IssueID:  issue.ID,
		ReviewID: review.ID,
	}

	comments, err := findCodeComments(ctx, opts, issue, currentUser, review, showOutdatedComments)
	if err != nil {
		return nil, err
	}

	for _, comment := range comments {
		if pathToLineToComment[comment.TreePath] == nil {
			pathToLineToComment[comment.TreePath] = make(map[int64][]*Comment)
		}
		pathToLineToComment[comment.TreePath][comment.Line] = append(pathToLineToComment[comment.TreePath][comment.Line], comment)
	}
	return pathToLineToComment, nil
}

func findCodeComments(ctx context.Context, opts FindCommentsOptions, issue *Issue, currentUser *user_model.User, review *Review, showOutdatedComments bool) ([]*Comment, error) {
	var comments CommentList
	if review == nil {
		review = &Review{ID: 0}
	}
	conds := opts.ToConds()

	if !showOutdatedComments && review.ID == 0 {
		conds = conds.And(builder.Eq{"invalidated": false})
	}

	e := db.GetEngine(ctx)
	if err := e.Where(conds).
		Asc("comment.created_unix").
		Asc("comment.id").
		Find(&comments); err != nil {
		return nil, err
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return nil, err
	}

	if err := comments.LoadPosters(ctx); err != nil {
		return nil, err
	}

	if err := comments.LoadAttachments(ctx); err != nil {
		return nil, err
	}

	if err := comments.loadResolveDoers(ctx); err != nil {
		return nil, err
	}

	if err := comments.loadReactions(ctx, issue.Repo); err != nil {
		return nil, err
	}

	// Find all reviews by ReviewID
	reviews := make(map[int64]*Review)
	ids := make([]int64, 0, len(comments))
	for _, comment := range comments {
		if comment.ReviewID != 0 {
			ids = append(ids, comment.ReviewID)
		}
	}
	if len(ids) > 0 {
		if err := e.In("id", ids).Find(&reviews); err != nil {
			return nil, err
		}
	}

	n := 0
	for _, comment := range comments {
		if re, ok := reviews[comment.ReviewID]; ok && re != nil {
			// If the review is pending only the author can see the comments (except if the review is set)
			if review.ID == 0 && re.Type == ReviewTypePending &&
				(currentUser == nil || currentUser.ID != re.ReviewerID) {
				continue
			}
			comment.Review = re
			comment.Issue = issue
		}
		comments[n] = comment
		n++

		if err := comment.RenderContent(ctx, issue.Repo); err != nil {
			return nil, err
		}
	}
	return comments[:n], nil
}

// FetchCodeCommentsByLine fetches the code comments for a given treePath and line number
func FetchCodeCommentsByLine(ctx context.Context, issue *Issue, currentUser *user_model.User, treePath string, line int64, showOutdatedComments bool) (CommentList, error) {
	opts := FindCommentsOptions{
		Type:     CommentTypeCode,
		IssueID:  issue.ID,
		TreePath: treePath,
		Line:     line,
	}
	return findCodeComments(ctx, opts, issue, currentUser, nil, showOutdatedComments)
}

type CodeSuggestionState struct {
	Reason string
}

func (c *Comment) CodeSuggestionCommitMessage() string {
	return "Apply suggestion from @" + c.Poster.Name
}

func (c *Comment) RenderContent(ctx context.Context, repo *repo_model.Repository) (err error) {
	rctx := renderhelper.NewRenderContextRepoComment(ctx, repo, renderhelper.RepoCommentOptions{
		FootnoteContextID: strconv.FormatInt(c.ID, 10),
	})
	if c.Type == CommentTypeCode && c.Line > 0 && c.CodeStartLine() > 0 && c.CodeCommitID() != "" {
		rctx.RenderOptions.CodeSuggestion = &markup.CodeSuggestionAnchor{
			CommitID: c.CodeCommitID(), FilePath: c.TreePath, StartLine: c.UnsignedCodeStartLine(), EndLine: c.UnsignedLine(),
		}
	}
	c.RenderedContent, err = markdown.RenderString(rctx, c.Content)
	return err
}

func GetPublishedCodeConversationRoot(ctx context.Context, issueID int64, treePath string, line int64) (*Comment, error) {
	first := new(Comment)
	has, err := db.GetEngine(ctx).Select("comment.*").Join("LEFT", "review", "review.id = comment.review_id").
		Where(builder.Eq{"comment.issue_id": issueID, "comment.tree_path": treePath, "comment.line": line, "comment.type": CommentTypeCode, "comment.invalidated": false}).
		And(builder.IsNull{"review.type"}.Or(builder.Neq{"review.type": ReviewTypePending})).
		Asc("comment.created_unix", "comment.id").Get(first)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, util.ErrNotExist
	}
	return first, nil
}

func FindCodeCommentsByIDs(ctx context.Context, issueID int64, ids []int64) (map[int64]*Comment, error) {
	comments := make(map[int64]*Comment, len(ids))
	err := db.GetEngine(ctx).In("id", ids).Where(builder.Eq{"issue_id": issueID, "type": CommentTypeCode}).Find(&comments)
	return comments, err
}
