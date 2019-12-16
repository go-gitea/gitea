// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/references"

	"github.com/unknwon/com"
	"xorm.io/xorm"
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
	RemoveOld   bool
}

func findOldCrossReferences(e Engine, issueID int64, commentID int64) ([]*Comment, error) {
	active := make([]*Comment, 0, 10)
	return active, e.Where("`ref_action` IN (?, ?, ?)", references.XRefActionNone, references.XRefActionCloses, references.XRefActionReopens).
		And("`ref_issue_id` = ?", issueID).
		And("`ref_comment_id` = ?", commentID).
		Find(&active)
}

func neuterCrossReferences(e Engine, issueID int64, commentID int64) error {
	active, err := findOldCrossReferences(e, issueID, commentID)
	if err != nil {
		return err
	}
	ids := make([]int64, len(active))
	for i, c := range active {
		ids[i] = c.ID
	}
	return neuterCrossReferencesIds(e, ids)
}

func neuterCrossReferencesIds(e Engine, ids []int64) error {
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

func (issue *Issue) addCrossReferences(e *xorm.Session, doer *User, removeOld bool) error {
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
		RemoveOld: removeOld,
	}
	return issue.createCrossReferences(e, ctx, issue.Title, issue.Content)
}

func (issue *Issue) createCrossReferences(e *xorm.Session, ctx *crossReferencesContext, plaincontent, mdcontent string) error {
	xreflist, err := ctx.OrigIssue.getCrossReferences(e, ctx, plaincontent, mdcontent)
	if err != nil {
		return err
	}
	if ctx.RemoveOld {
		var commentID int64
		if ctx.OrigComment != nil {
			commentID = ctx.OrigComment.ID
		}
		active, err := findOldCrossReferences(e, ctx.OrigIssue.ID, commentID)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(active))
		for _, c := range active {
			found := false
			for i, x := range xreflist {
				if x.Issue.ID == c.IssueID && x.Action == c.RefAction {
					found = true
					xreflist = append(xreflist[:i], xreflist[i+1:]...)
					break
				}
			}
			if !found {
				ids = append(ids, c.ID)
			}
		}
		if len(ids) > 0 {
			if err = neuterCrossReferencesIds(e, ids); err != nil {
				return err
			}
		}
	}
	for _, xref := range xreflist {
		var refCommentID int64
		if ctx.OrigComment != nil {
			refCommentID = ctx.OrigComment.ID
		}
		var opts = &CreateCommentOptions{
			Type:         ctx.Type,
			Doer:         ctx.Doer,
			Repo:         xref.Issue.Repo,
			Issue:        xref.Issue,
			RefRepoID:    ctx.OrigIssue.RepoID,
			RefIssueID:   ctx.OrigIssue.ID,
			RefCommentID: refCommentID,
			RefAction:    xref.Action,
			RefIsPull:    ctx.OrigIssue.IsPull,
		}
		_, err := createComment(e, opts)
		if err != nil {
			return err
		}
	}
	return nil
}

func (issue *Issue) getCrossReferences(e *xorm.Session, ctx *crossReferencesContext, plaincontent, mdcontent string) ([]*crossReference, error) {
	xreflist := make([]*crossReference, 0, 5)
	var (
		refRepo   *Repository
		refIssue  *Issue
		refAction references.XRefAction
		err       error
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
		if refIssue, refAction, err = ctx.OrigIssue.verifyReferencedIssue(e, ctx, refRepo, ref); err != nil {
			return nil, err
		}
		if refIssue != nil {
			xreflist = ctx.OrigIssue.updateCrossReferenceList(xreflist, &crossReference{
				Issue:  refIssue,
				Action: refAction,
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

// verifyReferencedIssue will check if the referenced issue exists, and whether the doer has permission to do what
func (issue *Issue) verifyReferencedIssue(e Engine, ctx *crossReferencesContext, repo *Repository,
	ref references.IssueReference) (*Issue, references.XRefAction, error) {

	refIssue := &Issue{RepoID: repo.ID, Index: ref.Index}
	refAction := ref.Action

	if has, _ := e.Get(refIssue); !has {
		return nil, references.XRefActionNone, nil
	}
	if err := refIssue.loadRepo(e); err != nil {
		return nil, references.XRefActionNone, err
	}

	// Close/reopen actions can only be set from pull requests to issues
	if refIssue.IsPull || !issue.IsPull {
		refAction = references.XRefActionNone
	}

	// Check doer permissions; set action to None if the doer can't change the destination
	if refIssue.RepoID != ctx.OrigIssue.RepoID || ref.Action != references.XRefActionNone {
		perm, err := getUserRepoPermission(e, refIssue.Repo, ctx.Doer)
		if err != nil {
			return nil, references.XRefActionNone, err
		}
		if !perm.CanReadIssuesOrPulls(refIssue.IsPull) {
			return nil, references.XRefActionNone, nil
		}
		// Accept close/reopening actions only if the poster is able to close the
		// referenced issue manually at this moment. The only exception is
		// the poster of a new PR referencing an issue on the same repo: then the merger
		// should be responsible for checking whether the reference should resolve.
		if ref.Action != references.XRefActionNone &&
			ctx.Doer.ID != refIssue.PosterID &&
			!perm.CanWriteIssuesOrPulls(refIssue.IsPull) &&
			(refIssue.RepoID != ctx.OrigIssue.RepoID || ctx.OrigComment != nil) {
			refAction = references.XRefActionNone
		}
	}

	return refIssue, refAction, nil
}

// _________                                       __
// \_   ___ \  ____   _____   _____   ____   _____/  |_
// /    \  \/ /  _ \ /     \ /     \_/ __ \ /    \   __\
// \     \___(  <_> )  Y Y  \  Y Y  \  ___/|   |  \  |
//  \______  /\____/|__|_|  /__|_|  /\___  >___|  /__|
//         \/             \/      \/     \/     \/
//

func (comment *Comment) addCrossReferences(e *xorm.Session, doer *User, removeOld bool) error {
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
		RemoveOld:   removeOld,
	}
	return comment.Issue.createCrossReferences(e, ctx, "", comment.Content)
}

func (comment *Comment) neuterCrossReferences(e Engine) error {
	return neuterCrossReferences(e, comment.IssueID, comment.ID)
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

// __________      .__  .__ __________                                     __
// \______   \__ __|  | |  |\______   \ ____  ________ __   ____   _______/  |_
//  |     ___/  |  \  | |  | |       _// __ \/ ____/  |  \_/ __ \ /  ___/\   __\
//  |    |   |  |  /  |_|  |_|    |   \  ___< <_|  |  |  /\  ___/ \___ \  |  |
//  |____|   |____/|____/____/____|_  /\___  >__   |____/  \___  >____  > |__|
//                                  \/     \/   |__|           \/     \/

// ResolveCrossReferences will return the list of references to close/reopen by this PR
func (pr *PullRequest) ResolveCrossReferences() ([]*Comment, error) {
	unfiltered := make([]*Comment, 0, 5)
	if err := x.
		Where("ref_repo_id = ? AND ref_issue_id = ?", pr.Issue.RepoID, pr.Issue.ID).
		In("ref_action", []references.XRefAction{references.XRefActionCloses, references.XRefActionReopens}).
		OrderBy("id").
		Find(&unfiltered); err != nil {
		return nil, fmt.Errorf("get reference: %v", err)
	}

	refs := make([]*Comment, 0, len(unfiltered))
	for _, ref := range unfiltered {
		found := false
		for i, r := range refs {
			if r.IssueID == ref.IssueID {
				// Keep only the latest
				refs[i] = ref
				found = true
				break
			}
		}
		if !found {
			refs = append(refs, ref)
		}
	}

	return refs, nil
}
