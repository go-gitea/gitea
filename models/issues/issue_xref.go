// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/references"
)

type crossReference struct {
	Issue  *Issue
	Action references.XRefAction
}

// crossReferencesContext is context to pass along findCrossReference functions
type crossReferencesContext struct {
	Type        CommentType
	Doer        *user_model.User
	OrigIssue   *Issue
	OrigComment *Comment
	RemoveOld   bool
}

func findOldCrossReferences(ctx context.Context, issueID, commentID int64) ([]*Comment, error) {
	active := make([]*Comment, 0, 10)
	return active, db.GetEngine(ctx).Where("`ref_action` IN (?, ?, ?)", references.XRefActionNone, references.XRefActionCloses, references.XRefActionReopens).
		And("`ref_issue_id` = ?", issueID).
		And("`ref_comment_id` = ?", commentID).
		Find(&active)
}

func neuterCrossReferences(ctx context.Context, issueID, commentID int64) error {
	active, err := findOldCrossReferences(ctx, issueID, commentID)
	if err != nil {
		return err
	}
	ids := make([]int64, len(active))
	for i, c := range active {
		ids[i] = c.ID
	}
	return neuterCrossReferencesIds(ctx, ids)
}

func neuterCrossReferencesIds(ctx context.Context, ids []int64) error {
	_, err := db.GetEngine(ctx).In("id", ids).Cols("`ref_action`").Update(&Comment{RefAction: references.XRefActionNeutered})
	return err
}

// AddCrossReferences add cross repositories references.
func (issue *Issue) AddCrossReferences(stdCtx context.Context, doer *user_model.User, removeOld bool) error {
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
	return issue.createCrossReferences(stdCtx, ctx, issue.Title, issue.Content)
}

func (issue *Issue) createCrossReferences(stdCtx context.Context, ctx *crossReferencesContext, plaincontent, mdcontent string) error {
	xreflist, err := ctx.OrigIssue.getCrossReferences(stdCtx, ctx, plaincontent, mdcontent)
	if err != nil {
		return err
	}
	if ctx.RemoveOld {
		var commentID int64
		if ctx.OrigComment != nil {
			commentID = ctx.OrigComment.ID
		}
		active, err := findOldCrossReferences(stdCtx, ctx.OrigIssue.ID, commentID)
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
			if err = neuterCrossReferencesIds(stdCtx, ids); err != nil {
				return err
			}
		}
	}
	for _, xref := range xreflist {
		var refCommentID int64
		if ctx.OrigComment != nil {
			refCommentID = ctx.OrigComment.ID
		}
		opts := &CreateCommentOptions{
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
		_, err := CreateComment(stdCtx, opts)
		if err != nil {
			return err
		}
	}
	return nil
}

func (issue *Issue) getCrossReferences(stdCtx context.Context, ctx *crossReferencesContext, plaincontent, mdcontent string) ([]*crossReference, error) {
	xreflist := make([]*crossReference, 0, 5)
	var (
		refRepo   *repo_model.Repository
		refIssue  *Issue
		refAction references.XRefAction
		err       error
	)

	allrefs := append(references.FindAllIssueReferences(plaincontent), references.FindAllIssueReferencesMarkdown(mdcontent)...)
	for _, ref := range allrefs {
		if ref.Owner == "" && ref.Name == "" {
			// Issues in the same repository
			if err := ctx.OrigIssue.LoadRepo(stdCtx); err != nil {
				return nil, err
			}
			refRepo = ctx.OrigIssue.Repo
		} else {
			// Issues in other repositories
			refRepo, err = repo_model.GetRepositoryByOwnerAndName(stdCtx, ref.Owner, ref.Name)
			if err != nil {
				if repo_model.IsErrRepoNotExist(err) {
					continue
				}
				return nil, err
			}
		}
		if refIssue, refAction, err = ctx.OrigIssue.verifyReferencedIssue(stdCtx, ctx, refRepo, ref); err != nil {
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
func (issue *Issue) verifyReferencedIssue(stdCtx context.Context, ctx *crossReferencesContext, repo *repo_model.Repository,
	ref references.IssueReference,
) (*Issue, references.XRefAction, error) {
	refIssue := &Issue{RepoID: repo.ID, Index: ref.Index}
	refAction := ref.Action
	e := db.GetEngine(stdCtx)

	if has, _ := e.Get(refIssue); !has {
		return nil, references.XRefActionNone, nil
	}
	if err := refIssue.LoadRepo(stdCtx); err != nil {
		return nil, references.XRefActionNone, err
	}

	// Close/reopen actions can only be set from pull requests to issues
	if refIssue.IsPull || !issue.IsPull {
		refAction = references.XRefActionNone
	}

	// Check doer permissions; set action to None if the doer can't change the destination
	if refIssue.RepoID != ctx.OrigIssue.RepoID || ref.Action != references.XRefActionNone {
		perm, err := access_model.GetUserRepoPermission(stdCtx, refIssue.Repo, ctx.Doer)
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

// AddCrossReferences add cross references
func (c *Comment) AddCrossReferences(stdCtx context.Context, doer *user_model.User, removeOld bool) error {
	if c.Type != CommentTypeCode && c.Type != CommentTypeComment {
		return nil
	}
	if err := c.LoadIssue(stdCtx); err != nil {
		return err
	}
	ctx := &crossReferencesContext{
		Type:        CommentTypeCommentRef,
		Doer:        doer,
		OrigIssue:   c.Issue,
		OrigComment: c,
		RemoveOld:   removeOld,
	}
	return c.Issue.createCrossReferences(stdCtx, ctx, "", c.Content)
}

func (c *Comment) neuterCrossReferences(ctx context.Context) error {
	return neuterCrossReferences(ctx, c.IssueID, c.ID)
}

// LoadRefComment loads comment that created this reference from database
func (c *Comment) LoadRefComment(ctx context.Context) (err error) {
	if c.RefComment != nil {
		return nil
	}
	c.RefComment, err = GetCommentByID(ctx, c.RefCommentID)
	return err
}

// LoadRefIssue loads comment that created this reference from database
func (c *Comment) LoadRefIssue(ctx context.Context) (err error) {
	if c.RefIssue != nil {
		return nil
	}
	c.RefIssue, err = GetIssueByID(ctx, c.RefIssueID)
	if err == nil {
		err = c.RefIssue.LoadRepo(ctx)
	}
	return err
}

// CommentTypeIsRef returns true if CommentType is a reference from another issue
func CommentTypeIsRef(t CommentType) bool {
	return t == CommentTypeCommentRef || t == CommentTypePullRef || t == CommentTypeIssueRef
}

// RefCommentLink returns the relative URL for the comment that created this reference
func (c *Comment) RefCommentLink(ctx context.Context) string {
	// Edge case for when the reference is inside the title or the description of the referring issue
	if c.RefCommentID == 0 {
		return c.RefIssueLink(ctx)
	}
	if err := c.LoadRefComment(ctx); err != nil { // Silently dropping errors :unamused:
		log.Error("LoadRefComment(%d): %v", c.RefCommentID, err)
		return ""
	}
	return c.RefComment.Link(ctx)
}

// RefIssueLink returns the relative URL of the issue where this reference was created
func (c *Comment) RefIssueLink(ctx context.Context) string {
	if err := c.LoadRefIssue(ctx); err != nil { // Silently dropping errors :unamused:
		log.Error("LoadRefIssue(%d): %v", c.RefCommentID, err)
		return ""
	}
	return c.RefIssue.Link()
}

// RefIssueTitle returns the title of the issue where this reference was created
func (c *Comment) RefIssueTitle(ctx context.Context) string {
	if err := c.LoadRefIssue(ctx); err != nil { // Silently dropping errors :unamused:
		log.Error("LoadRefIssue(%d): %v", c.RefCommentID, err)
		return ""
	}
	return c.RefIssue.Title
}

// RefIssueIdent returns the user friendly identity (e.g. "#1234") of the issue where this reference was created
func (c *Comment) RefIssueIdent(ctx context.Context) string {
	if err := c.LoadRefIssue(ctx); err != nil { // Silently dropping errors :unamused:
		log.Error("LoadRefIssue(%d): %v", c.RefCommentID, err)
		return ""
	}
	// FIXME: check this name for cross-repository references (#7901 if it gets merged)
	return fmt.Sprintf("#%d", c.RefIssue.Index)
}

// __________      .__  .__ __________                                     __
// \______   \__ __|  | |  |\______   \ ____  ________ __   ____   _______/  |_
//  |     ___/  |  \  | |  | |       _// __ \/ ____/  |  \_/ __ \ /  ___/\   __\
//  |    |   |  |  /  |_|  |_|    |   \  ___< <_|  |  |  /\  ___/ \___ \  |  |
//  |____|   |____/|____/____/____|_  /\___  >__   |____/  \___  >____  > |__|
//                                  \/     \/   |__|           \/     \/

// ResolveCrossReferences will return the list of references to close/reopen by this PR
func (pr *PullRequest) ResolveCrossReferences(ctx context.Context) ([]*Comment, error) {
	unfiltered := make([]*Comment, 0, 5)
	if err := db.GetEngine(ctx).
		Where("ref_repo_id = ? AND ref_issue_id = ?", pr.Issue.RepoID, pr.Issue.ID).
		In("ref_action", []references.XRefAction{references.XRefActionCloses, references.XRefActionReopens}).
		OrderBy("id").
		Find(&unfiltered); err != nil {
		return nil, fmt.Errorf("get reference: %w", err)
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
