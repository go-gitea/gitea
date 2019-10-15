// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/references"

	"github.com/go-xorm/xorm"
	"github.com/unknwon/com"
)

type crossReference struct {
	Issue  *Issue
	Action references.XRefAction
}

// crossReferencesContext is context to pass along findCrossReference functions
type crossReferencesContext struct {
	Type        CommentType
	Doer        *User
	OrigIssue   *Issue
	OrigComment *Comment
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

func neuterCrossReferences(e Engine, issueID int64, commentID int64) error {
	active := make([]*Comment, 0, 10)
	sess := e.Where("`ref_action` IN (?, ?, ?)", references.XRefActionNone, references.XRefActionCloses, references.XRefActionReopens)
	if issueID != 0 {
		sess = sess.And("`ref_issue_id` = ?", issueID)
	}
	if commentID != 0 {
		sess = sess.And("`ref_comment_id` = ?", commentID)
	}
	if err := sess.Find(&active); err != nil || len(active) == 0 {
		return err
	}
	ids := make([]int64, len(active))
	for i, c := range active {
		ids[i] = c.ID
	}
	_, err := e.In("id", ids).Cols("`ref_action`").Update(&Comment{RefAction: references.XRefActionNeutered})
	return err
}

// .___
// |   | ______ ________ __   ____
// |   |/  ___//  ___/  |  \_/ __ \
// |   |\___ \ \___ \|  |  /\  ___/
// |___/____  >____  >____/  \___  >
//          \/     \/            \/
//

func (issue *Issue) addCrossReferences(e *xorm.Session, doer *User) error {
	var commentType CommentType
	if issue.IsPull {
		commentType = CommentTypePullRef
	} else {
		commentType = CommentTypeIssueRef
	}
	ctx := &crossReferencesContext{
		Type:      commentType,
		Doer:      doer,
		OrigIssue: issue,
	}
	return issue.createCrossReferences(e, ctx, issue.Title, issue.Content)
}

func (issue *Issue) createCrossReferences(e *xorm.Session, ctx *crossReferencesContext, plaincontent, mdcontent string) error {
	xreflist, err := ctx.OrigIssue.getCrossReferences(e, ctx, plaincontent, mdcontent)
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

func (issue *Issue) getCrossReferences(e *xorm.Session, ctx *crossReferencesContext, plaincontent, mdcontent string) ([]*crossReference, error) {
	xreflist := make([]*crossReference, 0, 5)
	var (
		refRepo  *Repository
		refIssue *Issue
		err      error
	)

	allrefs := append(references.FindAllIssueReferences(plaincontent), references.FindAllIssueReferencesMarkdown(mdcontent)...)

	for _, ref := range allrefs {
		if ref.Owner == "" && ref.Name == "" {
			// Issues in the same repository
			if err := ctx.OrigIssue.loadRepo(e); err != nil {
				return nil, err
			}
			refRepo = ctx.OrigIssue.Repo
		} else {
			// Issues in other repositories
			refRepo, err = getRepositoryByOwnerAndName(e, ref.Owner, ref.Name)
			if err != nil {
				if IsErrRepoNotExist(err) {
					continue
				}
				return nil, err
			}
		}
		if refIssue, err = ctx.OrigIssue.findReferencedIssue(e, ctx, refRepo, ref.Index); err != nil {
			return nil, err
		}
		if refIssue != nil {
			xreflist = ctx.OrigIssue.updateCrossReferenceList(xreflist, &crossReference{
				Issue: refIssue,
				// FIXME: currently ignore keywords
				// Action: ref.Action,
				Action: references.XRefActionNone,
			})
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
			if xref.Action != references.XRefActionNone {
				list[i].Action = xref.Action
			}
			return list
		}
	}
	return append(list, xref)
}

func (issue *Issue) findReferencedIssue(e Engine, ctx *crossReferencesContext, repo *Repository, index int64) (*Issue, error) {
	refIssue := &Issue{RepoID: repo.ID, Index: index}
	if has, _ := e.Get(refIssue); !has {
		return nil, nil
	}
	if err := refIssue.loadRepo(e); err != nil {
		return nil, err
	}
	// Check user permissions
	if refIssue.RepoID != ctx.OrigIssue.RepoID {
		perm, err := getUserRepoPermission(e, refIssue.Repo, ctx.Doer)
		if err != nil {
			return nil, err
		}
		if !perm.CanReadIssuesOrPulls(refIssue.IsPull) {
			return nil, nil
		}
	}
	return refIssue, nil
}

func (issue *Issue) neuterCrossReferences(e Engine) error {
	return neuterCrossReferences(e, issue.ID, 0)
}

// _________                                       __
// \_   ___ \  ____   _____   _____   ____   _____/  |_
// /    \  \/ /  _ \ /     \ /     \_/ __ \ /    \   __\
// \     \___(  <_> )  Y Y  \  Y Y  \  ___/|   |  \  |
//  \______  /\____/|__|_|  /__|_|  /\___  >___|  /__|
//         \/             \/      \/     \/     \/
//

func (comment *Comment) addCrossReferences(e *xorm.Session, doer *User) error {
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
	return comment.Issue.createCrossReferences(e, ctx, "", comment.Content)
}

func (comment *Comment) neuterCrossReferences(e Engine) error {
	return neuterCrossReferences(e, 0, comment.ID)
}

// LoadRefComment loads comment that created this reference from database
func (comment *Comment) LoadRefComment() (err error) {
	if comment.RefComment != nil {
		return nil
	}
	comment.RefComment, err = GetCommentByID(comment.RefCommentID)
	return
}

// LoadRefIssue loads comment that created this reference from database
func (comment *Comment) LoadRefIssue() (err error) {
	if comment.RefIssue != nil {
		return nil
	}
	comment.RefIssue, err = GetIssueByID(comment.RefIssueID)
	if err == nil {
		err = comment.RefIssue.loadRepo(x)
	}
	return
}

// CommentTypeIsRef returns true if CommentType is a reference from another issue
func CommentTypeIsRef(t CommentType) bool {
	return t == CommentTypeCommentRef || t == CommentTypePullRef || t == CommentTypeIssueRef
}

// RefCommentHTMLURL returns the HTML URL for the comment that created this reference
func (comment *Comment) RefCommentHTMLURL() string {
	if err := comment.LoadRefComment(); err != nil { // Silently dropping errors :unamused:
		log.Error("LoadRefComment(%d): %v", comment.RefCommentID, err)
		return ""
	}
	return comment.RefComment.HTMLURL()
}

// RefIssueHTMLURL returns the HTML URL of the issue where this reference was created
func (comment *Comment) RefIssueHTMLURL() string {
	if err := comment.LoadRefIssue(); err != nil { // Silently dropping errors :unamused:
		log.Error("LoadRefIssue(%d): %v", comment.RefCommentID, err)
		return ""
	}
	return comment.RefIssue.HTMLURL()
}

// RefIssueTitle returns the title of the issue where this reference was created
func (comment *Comment) RefIssueTitle() string {
	if err := comment.LoadRefIssue(); err != nil { // Silently dropping errors :unamused:
		log.Error("LoadRefIssue(%d): %v", comment.RefCommentID, err)
		return ""
	}
	return comment.RefIssue.Title
}

// RefIssueIdent returns the user friendly identity (e.g. "#1234") of the issue where this reference was created
func (comment *Comment) RefIssueIdent() string {
	if err := comment.LoadRefIssue(); err != nil { // Silently dropping errors :unamused:
		log.Error("LoadRefIssue(%d): %v", comment.RefCommentID, err)
		return ""
	}
	// FIXME: check this name for cross-repository references (#7901 if it gets merged)
	return "#" + com.ToStr(comment.RefIssue.Index)
}
