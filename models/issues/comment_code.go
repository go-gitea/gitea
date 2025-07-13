// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"strconv"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/markup/markdown"

	"xorm.io/builder"
)

// CodeComments represents comments on code by using this structure: FILENAME -> LINE (+ == proposed; - == previous) -> COMMENTS
type CodeComments map[string][]*Comment

func (cc CodeComments) AllComments() []*Comment {
	var allComments []*Comment
	for _, comments := range cc {
		allComments = append(allComments, comments...)
	}
	return allComments
}

// FetchCodeComments will return a 2d-map: ["Path"]["Line"] = Comments at line
func FetchCodeComments(ctx context.Context, repo *repo_model.Repository, issueID int64, currentUser *user_model.User, showOutdatedComments bool) (CodeComments, error) {
	return fetchCodeCommentsByReview(ctx, repo, issueID, currentUser, nil, showOutdatedComments)
}

func fetchCodeCommentsByReview(ctx context.Context, repo *repo_model.Repository, issueID int64, currentUser *user_model.User, review *Review, showOutdatedComments bool) (CodeComments, error) {
	codeCommentsPathMap := make(CodeComments)
	if review == nil {
		review = &Review{ID: 0}
	}
	opts := FindCommentsOptions{
		Type:     CommentTypeCode,
		IssueID:  issueID,
		ReviewID: review.ID,
	}

	comments, err := FindCodeComments(ctx, opts, repo, currentUser, review, showOutdatedComments)
	if err != nil {
		return nil, err
	}

	for _, comment := range comments {
		codeCommentsPathMap[comment.TreePath] = append(codeCommentsPathMap[comment.TreePath], comment)
	}
	return codeCommentsPathMap, nil
}

func FindCodeComments(ctx context.Context, opts FindCommentsOptions, repo *repo_model.Repository, currentUser *user_model.User, review *Review, showOutdatedComments bool) ([]*Comment, error) {
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

	if err := comments.LoadPosters(ctx); err != nil {
		return nil, err
	}

	if err := comments.LoadAttachments(ctx); err != nil {
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
		}
		comments[n] = comment
		n++

		if err := comment.LoadResolveDoer(ctx); err != nil {
			return nil, err
		}

		if err := comment.LoadReactions(ctx, repo); err != nil {
			return nil, err
		}

		var err error
		rctx := renderhelper.NewRenderContextRepoComment(ctx, repo, renderhelper.RepoCommentOptions{
			FootnoteContextID: strconv.FormatInt(comment.ID, 10),
		})
		if comment.RenderedContent, err = markdown.RenderString(rctx, comment.Content); err != nil {
			return nil, err
		}
	}
	return comments[:n], nil
}
