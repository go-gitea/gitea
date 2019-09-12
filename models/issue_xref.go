// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"regexp"
	"strconv"

	"github.com/go-xorm/xorm"
)

var (
	// TODO: Unify all regexp treatment of cross references in one place

	// issueNumericPattern matches string that references to a numeric issue, e.g. #1287
	issueNumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)(?:#)([0-9]+)(?:\s|$|\)|\]|:|\.(\s|$))`)
	// crossReferenceIssueNumericPattern matches string that references a numeric issue in a different repository
	// e.g. gogits/gogs#12345
	crossReferenceIssueNumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([0-9a-zA-Z-_\.]+)/([0-9a-zA-Z-_\.]+)#([0-9]+)(?:\s|$|\)|\]|\.(\s|$))`)
)

// XRefAction represents the kind of effect a cross reference has once is resolved
type XRefAction int64

const (
	// XRefActionNone means the cross-reference is a mention (commit, etc.)
	XRefActionNone XRefAction = iota		// 0
	// XRefActionCloses means the cross-reference should close an issue if it is resolved
	XRefActionCloses						// 1 - not implemented yet
	// XRefActionReopens means the cross-reference should reopen an issue if it is resolved
	XRefActionReopens						// 2 - Not implemented yet
	// XRefActionNeutered means the cross-reference will no longer affect the source
	XRefActionNeutered						// 3
)

type crossReference struct {
	Issue  *Issue
	Action XRefAction
}

// crossReferencesContext is context to pass along findCrossReference functions
type crossReferencesContext struct {
	Type        CommentType
	Doer        *User
	OrigIssue   *Issue
	OrigComment *Comment
}

func (issue *Issue) addIssueReferences(e *xorm.Session, doer *User) error {
	ctx := &crossReferencesContext{
		Type:      CommentTypeIssueRef,
		Doer:      doer,
		OrigIssue: issue,
	}
	return issue.findCrossReferences(e, ctx, issue.Title + "\n" + issue.Content)
}

func (comment *Comment) addCommentReferences(e *xorm.Session, doer *User) error {
	if comment.Type != CommentTypeCode && comment.Type != CommentTypeComment {
		return nil
	}
	if err := comment.loadIssue(e); err != nil {
		return err
	}
	ctx := &crossReferencesContext{
		Type:        CommentTypeCommentRef,
		Doer:        doer,
		OrigIssue:   comment.Issue,
		OrigComment: comment,
	}
	return comment.Issue.findCrossReferences(e, ctx, comment.Content)
}

func (issue *Issue) findCrossReferences(e *xorm.Session, ctx *crossReferencesContext, content string) error {
	xreflist, err := ctx.OrigIssue.getCrossReferences(e, ctx, content)
	if err != nil {
		return err
	}
	for _, xref := range xreflist {
		if err = newCrossReference(e, ctx, xref); err != nil {
			return err
		}
	}
	return nil
}

func (issue *Issue) getCrossReferences(e Engine, ctx *crossReferencesContext, content string) ([]*crossReference, error) {
	xreflist := make([]*crossReference, 0, 5)
	var xref *crossReference

	// Issues in the same repository
	// FIXME: Should we support IssueNameStyleAlphanumeric?
	matches := issueNumericPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if index, err := strconv.ParseInt(match[1], 10, 64); err == nil {
			if err = ctx.OrigIssue.loadRepo(e); err != nil {
				return nil, err
			}
			if xref, err = ctx.OrigIssue.isValidCommentReference(e, ctx, issue.Repo, index); err != nil {
				return nil, err
			}
			if xref != nil {
				xreflist = ctx.OrigIssue.updateCrossReferenceList(xreflist, xref)
			}
		}
	}

	// Issues in other repositories
	matches = crossReferenceIssueNumericPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if index, err := strconv.ParseInt(match[3], 10, 64); err == nil {
			repo, err := GetRepositoryByOwnerAndName(match[1], match[2])
			if err != nil {
				if IsErrRepoNotExist(err) {
					continue
				}
				return nil, err
			}
			if err = ctx.OrigIssue.loadRepo(e); err != nil {
				return nil, err
			}
			if xref, err = issue.isValidCommentReference(e, ctx, repo, index); err != nil {
				return nil, err
			}
			if xref != nil {
				xreflist = issue.updateCrossReferenceList(xreflist, xref)
			}
		}
	}

	return xreflist, nil
}

func (issue *Issue) updateCrossReferenceList(list []*crossReference, xref *crossReference) []*crossReference {
	if xref.Issue.ID == issue.ID {
		return list
	}
	for i, r := range list {
		if r.Issue.ID == xref.Issue.ID {
			if xref.Action != XRefActionNone {
				list[i].Action = xref.Action
			}
			return list
		}
	}
	return append(list, xref)
}

func (issue *Issue) isValidCommentReference(e Engine, ctx *crossReferencesContext, repo *Repository, index int64) (*crossReference, error) {
	refIssue := &Issue{RepoID: repo.ID, Index: index}
	if has, _ := e.Get(refIssue); !has {
		return nil, nil
	}
	if err := refIssue.loadRepo(e); err != nil {
		return nil, err
	}
	// Check user permissions
	if refIssue.Repo.ID != ctx.OrigIssue.Repo.ID {
		perm, err := getUserRepoPermission(e, refIssue.Repo, ctx.Doer)
		if err != nil {
			return nil, err
		}
		if !perm.CanReadIssuesOrPulls(refIssue.IsPull) {
			return nil, nil
		}
	}
	return &crossReference{
		Issue:  refIssue,
		Action: XRefActionNone,
	}, nil
}

func newCrossReference(e *xorm.Session, ctx *crossReferencesContext, xref *crossReference) error {
	var refCommentID int64
	if ctx.OrigComment != nil {
		refCommentID = ctx.OrigComment.ID
	}
	_, err := createComment(e, &CreateCommentOptions{
		Type:         ctx.Type,
		Doer:         ctx.Doer,
		Repo:         xref.Issue.Repo,
		Issue:        xref.Issue,
		RefRepoID:    ctx.OrigIssue.RepoID,
		RefIssueID:   ctx.OrigIssue.ID,
		RefCommentID: refCommentID,
		RefAction:    xref.Action,
		RefIsPull:    xref.Issue.IsPull,
	})
	return err
}

func (issue *Issue) neuterReferencingComments(e Engine) error {
	return neuterReferencingComments(e, issue.ID, 0)
}

func (comment *Comment) neuterReferencingComments(e Engine) error {
	return neuterReferencingComments(e, 0, comment.ID)
}

func neuterReferencingComments(e Engine, issueID int64, commentID int64) error {
	active := make([]*Comment, 0, 10)
	sess := e.Where("`ref_action` IN (?, ?, ?)", XRefActionNone, XRefActionCloses, XRefActionReopens)
	if issueID != 0 {
		sess = sess.And("`ref_issue_id` = ?", issueID)
	}
	if commentID != 0 {
		sess = sess.And("`ref_comment_id` = ?", commentID)
	}
	err := sess.Find(&active)
	if err != nil || len(active) == 0 {
		return err
	}
	ids := make([]int64, len(active))
	for i, c := range active {
		ids[i] = c.ID
	}
	_, err = e.In("id", ids).Cols("`ref_action`").Update(&Comment{RefAction: XRefActionNeutered})
	return err
}
