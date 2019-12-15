// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"testing"

	"code.gitea.io/gitea/modules/references"

	"github.com/stretchr/testify/assert"
)

func TestXRef_AddCrossReferences(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// Issue #1 to test against
	itarget := testCreateIssue(t, 1, 2, "title1", "content1", false)

	// PR to close issue #1
	content := fmt.Sprintf("content2, closes #%d", itarget.Index)
	pr := testCreateIssue(t, 1, 2, "title2", content, true)
	ref := AssertExistsAndLoadBean(t, &Comment{IssueID: itarget.ID, RefIssueID: pr.ID, RefCommentID: 0}).(*Comment)
	assert.Equal(t, CommentTypePullRef, ref.Type)
	assert.Equal(t, pr.RepoID, ref.RefRepoID)
	assert.Equal(t, true, ref.RefIsPull)
	assert.Equal(t, references.XRefActionCloses, ref.RefAction)

	// Comment on PR to reopen issue #1
	content = fmt.Sprintf("content2, reopens #%d", itarget.Index)
	c := testCreateComment(t, 1, 2, pr.ID, content)
	ref = AssertExistsAndLoadBean(t, &Comment{IssueID: itarget.ID, RefIssueID: pr.ID, RefCommentID: c.ID}).(*Comment)
	assert.Equal(t, CommentTypeCommentRef, ref.Type)
	assert.Equal(t, pr.RepoID, ref.RefRepoID)
	assert.Equal(t, true, ref.RefIsPull)
	assert.Equal(t, references.XRefActionReopens, ref.RefAction)

	// Issue mentioning issue #1
	content = fmt.Sprintf("content3, mentions #%d", itarget.Index)
	i := testCreateIssue(t, 1, 2, "title3", content, false)
	ref = AssertExistsAndLoadBean(t, &Comment{IssueID: itarget.ID, RefIssueID: i.ID, RefCommentID: 0}).(*Comment)
	assert.Equal(t, CommentTypeIssueRef, ref.Type)
	assert.Equal(t, pr.RepoID, ref.RefRepoID)
	assert.Equal(t, false, ref.RefIsPull)
	assert.Equal(t, references.XRefActionNone, ref.RefAction)

	// Issue #4 to test against
	itarget = testCreateIssue(t, 3, 3, "title4", "content4", false)

	// Cross-reference to issue #4 by admin
	content = fmt.Sprintf("content5, mentions user3/repo3#%d", itarget.Index)
	i = testCreateIssue(t, 2, 1, "title5", content, false)
	ref = AssertExistsAndLoadBean(t, &Comment{IssueID: itarget.ID, RefIssueID: i.ID, RefCommentID: 0}).(*Comment)
	assert.Equal(t, CommentTypeIssueRef, ref.Type)
	assert.Equal(t, i.RepoID, ref.RefRepoID)
	assert.Equal(t, false, ref.RefIsPull)
	assert.Equal(t, references.XRefActionNone, ref.RefAction)

	// Cross-reference to issue #4 with no permission
	content = fmt.Sprintf("content6, mentions user3/repo3#%d", itarget.Index)
	i = testCreateIssue(t, 4, 5, "title6", content, false)
	AssertNotExistsBean(t, &Comment{IssueID: itarget.ID, RefIssueID: i.ID, RefCommentID: 0})
}

func TestXRef_NeuterCrossReferences(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// Issue #1 to test against
	itarget := testCreateIssue(t, 1, 2, "title1", "content1", false)

	// Issue mentioning issue #1
	title := fmt.Sprintf("title2, mentions #%d", itarget.Index)
	i := testCreateIssue(t, 1, 2, title, "content2", false)
	ref := AssertExistsAndLoadBean(t, &Comment{IssueID: itarget.ID, RefIssueID: i.ID, RefCommentID: 0}).(*Comment)
	assert.Equal(t, CommentTypeIssueRef, ref.Type)
	assert.Equal(t, references.XRefActionNone, ref.RefAction)

	d := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	i.Title = "title2, no mentions"
	assert.NoError(t, i.ChangeTitle(d, title))

	ref = AssertExistsAndLoadBean(t, &Comment{IssueID: itarget.ID, RefIssueID: i.ID, RefCommentID: 0}).(*Comment)
	assert.Equal(t, CommentTypeIssueRef, ref.Type)
	assert.Equal(t, references.XRefActionNeutered, ref.RefAction)
}

func TestXRef_ResolveCrossReferences(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	d := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)

	i1 := testCreateIssue(t, 1, 2, "title1", "content1", false)
	i2 := testCreateIssue(t, 1, 2, "title2", "content2", false)
	i3 := testCreateIssue(t, 1, 2, "title3", "content3", false)
	_, err := i3.ChangeStatus(d, true)
	assert.NoError(t, err)

	pr := testCreatePR(t, 1, 2, "titlepr", fmt.Sprintf("closes #%d", i1.Index))
	rp := AssertExistsAndLoadBean(t, &Comment{IssueID: i1.ID, RefIssueID: pr.Issue.ID, RefCommentID: 0}).(*Comment)

	c1 := testCreateComment(t, 1, 2, pr.Issue.ID, fmt.Sprintf("closes #%d", i2.Index))
	r1 := AssertExistsAndLoadBean(t, &Comment{IssueID: i2.ID, RefIssueID: pr.Issue.ID, RefCommentID: c1.ID}).(*Comment)

	// Must be ignored
	c2 := testCreateComment(t, 1, 2, pr.Issue.ID, fmt.Sprintf("mentions #%d", i2.Index))
	AssertExistsAndLoadBean(t, &Comment{IssueID: i2.ID, RefIssueID: pr.Issue.ID, RefCommentID: c2.ID})

	// Must be superseded by c4/r4
	c3 := testCreateComment(t, 1, 2, pr.Issue.ID, fmt.Sprintf("reopens #%d", i3.Index))
	AssertExistsAndLoadBean(t, &Comment{IssueID: i3.ID, RefIssueID: pr.Issue.ID, RefCommentID: c3.ID})

	c4 := testCreateComment(t, 1, 2, pr.Issue.ID, fmt.Sprintf("closes #%d", i3.Index))
	r4 := AssertExistsAndLoadBean(t, &Comment{IssueID: i3.ID, RefIssueID: pr.Issue.ID, RefCommentID: c4.ID}).(*Comment)

	refs, err := pr.ResolveCrossReferences()
	assert.NoError(t, err)
	assert.Len(t, refs, 3)
	assert.Equal(t, rp.ID, refs[0].ID, "bad ref rp: %+v", refs[0])
	assert.Equal(t, r1.ID, refs[1].ID, "bad ref r1: %+v", refs[1])
	assert.Equal(t, r4.ID, refs[2].ID, "bad ref r4: %+v", refs[2])
}

func testCreateIssue(t *testing.T, repo, doer int64, title, content string, ispull bool) *Issue {
	r := AssertExistsAndLoadBean(t, &Repository{ID: repo}).(*Repository)
	d := AssertExistsAndLoadBean(t, &User{ID: doer}).(*User)
	i := &Issue{RepoID: r.ID, PosterID: d.ID, Poster: d, Title: title, Content: content, IsPull: ispull}

	sess := x.NewSession()
	defer sess.Close()
	assert.NoError(t, sess.Begin())
	_, err := sess.SetExpr("`index`", "coalesce(MAX(`index`),0)+1").Where("repo_id=?", repo).Insert(i)
	assert.NoError(t, err)
	i, err = getIssueByID(sess, i.ID)
	assert.NoError(t, err)
	assert.NoError(t, i.addCrossReferences(sess, d, false))
	assert.NoError(t, sess.Commit())
	return i
}

func testCreatePR(t *testing.T, repo, doer int64, title, content string) *PullRequest {
	r := AssertExistsAndLoadBean(t, &Repository{ID: repo}).(*Repository)
	d := AssertExistsAndLoadBean(t, &User{ID: doer}).(*User)
	i := &Issue{RepoID: r.ID, PosterID: d.ID, Poster: d, Title: title, Content: content, IsPull: true}
	pr := &PullRequest{HeadRepoID: repo, BaseRepoID: repo, HeadBranch: "head", BaseBranch: "base", Status: PullRequestStatusMergeable}
	assert.NoError(t, NewPullRequest(r, i, nil, nil, pr))
	pr.Issue = i
	return pr
}

func testCreateComment(t *testing.T, repo, doer, issue int64, content string) *Comment {
	d := AssertExistsAndLoadBean(t, &User{ID: doer}).(*User)
	i := AssertExistsAndLoadBean(t, &Issue{ID: issue}).(*Issue)
	c := &Comment{Type: CommentTypeComment, PosterID: doer, Poster: d, IssueID: issue, Issue: i, Content: content}

	sess := x.NewSession()
	defer sess.Close()
	assert.NoError(t, sess.Begin())
	_, err := sess.Insert(c)
	assert.NoError(t, err)
	assert.NoError(t, c.addCrossReferences(sess, d, false))
	assert.NoError(t, sess.Commit())
	return c
}
