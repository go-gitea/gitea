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
	issueNumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)(#[0-9]+)(?:\s|$|\)|\]|:|\.(\s|$))`)
	// crossReferenceIssueNumericPattern matches string that references a numeric issue in a different repository
	// e.g. gogits/gogs#12345
	// crossReferenceIssueNumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([0-9a-zA-Z-_\.]+/[0-9a-zA-Z-_\.]+#[0-9]+)(?:\s|$|\)|\]|\.(\s|$))`)
)

// XRefAction represents the kind of effect a cross reference has once is resolved
type XRefAction int64

const (
	// XRefActionNone means the cross-reference is a mention (commit, etc.)
	XRefActionNone			XRefAction = iota
	// XRefActionCloses means the cross-reference should close an issue if it is resolved
	XRefActionCloses		// Not implemented yet
	// XRefActionReopens means the cross-reference should reopen an issue if it is resolved
	XRefActionReopens		// Not implemented yet
)

type crossReference struct {
	Issue		*Issue
	Action		XRefAction
}

// ParseReferencesOptions represents a comment or issue that might make references to other issues
type ParseReferencesOptions struct {
	Type        CommentType
	Doer        *User
	OrigIssue   *Issue
	OrigComment *Comment
}

func (issue *Issue) parseCommentReferences(e *xorm.Session, refopts *ParseReferencesOptions, content string) error {
	xreflist, err := issue.getCrossReferences(e, refopts, content)
	if err != nil {
		return err
	}
	for _, xref := range xreflist {
		if err = addCommentReference(e, refopts, xref); err != nil {
			return err
		}
	}
	return nil
}

func (issue *Issue) getCrossReferences(e *xorm.Session, refopts *ParseReferencesOptions, content string) ([]*crossReference, error) {
	xreflist := make([]*crossReference,0,5)
	var xref *crossReference

	// Issues in the same repository
	// FIXME: Should we support IssueNameStyleAlphanumeric?
	matches := issueNumericPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if i, err := strconv.ParseInt(match[1][1:], 10, 64); err == nil {
			if err = refopts.OrigIssue.loadRepo(e); err != nil {
				return nil, err
			}
			if xref, err = issue.checkCommentReference(e, refopts, issue.Repo, i); err != nil {
				return nil, err
			}
			if xref != nil {
				xreflist = issue.updateCrossReferenceList(xreflist, xref)
			}
		}
	}

	// Issues in other repositories
	// GAP: TODO: use crossReferenceIssueNumericPattern to parse references to other repos

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

func (issue *Issue) checkCommentReference(e *xorm.Session, refopts *ParseReferencesOptions, repo *Repository, index int64) (*crossReference, error) {
	refIssue := &Issue{RepoID: repo.ID, Index: index}
	if has, _ := e.Get(refIssue); !has {
		return nil, nil
	}
	if err := refIssue.loadRepo(e); err != nil {
		return nil, err
	}	
	// Check user permissions
	if refIssue.Repo.ID != refopts.OrigIssue.Repo.ID {
		perm, err := getUserRepoPermission(e, refIssue.Repo, refopts.Doer)
		if err != nil {
			return nil, err
		}
		if !perm.CanReadIssuesOrPulls(refIssue.IsPull) {
			return nil, nil
		}
	}
	return &crossReference {
		Issue: refIssue,
		Action: XRefActionNone,
	}, nil
}

func addCommentReference(e *xorm.Session, refopts *ParseReferencesOptions, xref *crossReference) error {
	var refCommentID int64
	if refopts.OrigComment != nil {
		refCommentID = refopts.OrigComment.ID
	}
	_, err := createComment(e, &CreateCommentOptions{
		Type:         refopts.Type,
		Doer:         refopts.Doer,
		Repo:         xref.Issue.Repo,
		Issue:        xref.Issue,
		RefIssueID:   refopts.OrigIssue.ID,
		RefCommentID: refCommentID,
		RefAction:    xref.Action,
	})
	return err
}
