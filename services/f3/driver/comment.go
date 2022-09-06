// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package driver

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
	comment_service "code.gitea.io/gitea/services/comments"

	"lab.forgefriends.org/friendlyforgeformat/gof3/forges/common"
	"lab.forgefriends.org/friendlyforgeformat/gof3/format"
	"lab.forgefriends.org/friendlyforgeformat/gof3/util"
)

type Comment struct {
	issues_model.Comment
}

func CommentConverter(f *issues_model.Comment) *Comment {
	return &Comment{
		Comment: *f,
	}
}

func (o Comment) GetID() int64 {
	return o.Comment.ID
}

func (o *Comment) SetID(id int64) {
	o.Comment.ID = id
}

func (o *Comment) IsNil() bool {
	return o.ID == 0
}

func (o *Comment) Equals(other *Comment) bool {
	return o.Comment.ID == other.Comment.ID
}

func (o *Comment) ToFormat() *format.Comment {
	return &format.Comment{
		Common:      format.Common{Index: o.Comment.ID},
		IssueIndex:  o.Comment.IssueID,
		PosterID:    o.Comment.PosterID,
		PosterName:  o.Comment.Poster.Name,
		PosterEmail: o.Comment.Poster.Email,
		Content:     o.Comment.Content,
		Created:     o.Comment.CreatedUnix.AsTime(),
		Updated:     o.Comment.UpdatedUnix.AsTime(),
	}
}

func (o *Comment) FromFormat(comment *format.Comment) {
	*o = Comment{
		Comment: issues_model.Comment{
			ID:       comment.Index,
			PosterID: comment.PosterID,
			Poster: &user_model.User{
				ID:    comment.PosterID,
				Name:  comment.PosterName,
				Email: comment.PosterEmail,
			},
			Content:     comment.Content,
			CreatedUnix: timeutil.TimeStamp(comment.Created.Unix()),
			UpdatedUnix: timeutil.TimeStamp(comment.Updated.Unix()),
		},
	}
}

type CommentProvider struct {
	g *Gitea
}

func (o *CommentProvider) ToFormat(comment *Comment) *format.Comment {
	return comment.ToFormat()
}

func (o *CommentProvider) FromFormat(f *format.Comment) *Comment {
	var comment Comment
	comment.FromFormat(f)
	return &comment
}

func (o *CommentProvider) GetObjects(user *User, project *Project, commentable common.ContainerObjectInterface, page int) []*Comment {
	comments, err := issues_model.FindComments(o.g.ctx, &issues_model.FindCommentsOptions{
		ListOptions: db.ListOptions{Page: page, PageSize: o.g.perPage},
		RepoID:      project.GetID(),
		IssueID:     commentable.GetID(),
		Type:        issues_model.CommentTypeComment,
	})
	if err != nil {
		panic(fmt.Errorf("error while listing comment: %v", err))
	}

	return util.ConvertMap[*issues_model.Comment, *Comment](comments, CommentConverter)
}

func (o *CommentProvider) ProcessObject(user *User, project *Project, commentable common.ContainerObjectInterface, comment *Comment) {
	if err := comment.LoadIssue(); err != nil {
		panic(err)
	}
	if err := comment.LoadPoster(); err != nil {
		panic(err)
	}
}

func (o *CommentProvider) Get(user *User, project *Project, commentable common.ContainerObjectInterface, comment *Comment) *Comment {
	id := comment.GetID()
	c, err := issues_model.GetCommentByID(o.g.ctx, id)
	if issues_model.IsErrCommentNotExist(err) {
		return &Comment{}
	}
	if err != nil {
		panic(err)
	}

	co := CommentConverter(c)
	o.ProcessObject(user, project, commentable, co)
	return co
}

func (o *CommentProvider) Put(user *User, project *Project, commentable common.ContainerObjectInterface, comment *Comment) *Comment {
	var issue *issues_model.Issue
	switch c := commentable.(type) {
	case *PullRequest:
		issue = c.PullRequest.Issue
	case *Issue:
		issue = &c.Issue
	default:
		panic(fmt.Errorf("unexpected type %T", commentable))
	}
	c, err := comment_service.CreateIssueComment(o.g.GetDoer(), &project.Repository, issue, comment.Content, nil)
	if err != nil {
		panic(err)
	}
	return o.Get(user, project, commentable, CommentConverter(c))
}

func (o *CommentProvider) Delete(user *User, project *Project, commentable common.ContainerObjectInterface, comment *Comment) *Comment {
	c := o.Get(user, project, commentable, comment)
	if !c.IsNil() {
		err := issues_model.DeleteComment(o.g.ctx, &c.Comment)
		if err != nil {
			panic(err)
		}
	}
	return c
}
