// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"html/template"
	"testing"

	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

// TODO TestGetLabelTemplateFile

func TestLabel_APIFormat(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	label := AssertExistsAndLoadBean(t, &Label{ID: 1}).(*Label)
	assert.Equal(t, api.Label{
		ID:    label.ID,
		Name:  label.Name,
		Color: "abcdef",
	}, *label.APIFormat())
}

func TestLabel_CalOpenIssues(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	label := AssertExistsAndLoadBean(t, &Label{ID: 1}).(*Label)
	label.CalOpenIssues()
	assert.EqualValues(t, 2, label.NumOpenIssues)
}

func TestLabel_ForegroundColor(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	label := AssertExistsAndLoadBean(t, &Label{ID: 1}).(*Label)
	assert.Equal(t, template.CSS("#000"), label.ForegroundColor())

	label = AssertExistsAndLoadBean(t, &Label{ID: 2}).(*Label)
	assert.Equal(t, template.CSS("#fff"), label.ForegroundColor())
}

func TestNewLabels(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	labels := []*Label{
		{RepoID: 2, Name: "labelName2", Color: "#123456"},
		{RepoID: 3, Name: "labelName3", Color: "#234567"},
	}
	for _, label := range labels {
		AssertNotExistsBean(t, label)
	}
	assert.NoError(t, NewLabels(labels...))
	for _, label := range labels {
		AssertExistsAndLoadBean(t, label, Cond("id = ?", label.ID))
	}
	CheckConsistencyFor(t, &Label{}, &Repository{})
}

func TestGetLabelByID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	label, err := GetLabelByID(1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, label.ID)

	_, err = GetLabelByID(NonexistentID)
	assert.True(t, IsErrLabelNotExist(err))
}

func TestGetLabelInRepoByName(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	label, err := GetLabelInRepoByName(1, "label1")
	assert.NoError(t, err)
	assert.EqualValues(t, 1, label.ID)
	assert.Equal(t, "label1", label.Name)

	_, err = GetLabelInRepoByName(1, "")
	assert.True(t, IsErrLabelNotExist(err))

	_, err = GetLabelInRepoByName(NonexistentID, "nonexistent")
	assert.True(t, IsErrLabelNotExist(err))
}

func TestGetLabelInRepoByNames(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	labelIDs, err := GetLabelIDsInRepoByNames(1, []string{"label1", "label2"})
	assert.NoError(t, err)

	assert.Len(t, labelIDs, 2)

	assert.Equal(t, int64(1), labelIDs[0])
	assert.Equal(t, int64(2), labelIDs[1])
}

func TestGetLabelInRepoByNamesDiscardsNonExistentLabels(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	// label3 doesn't exists.. See labels.yml
	labelIDs, err := GetLabelIDsInRepoByNames(1, []string{"label1", "label2", "label3"})
	assert.NoError(t, err)

	assert.Len(t, labelIDs, 2)

	assert.Equal(t, int64(1), labelIDs[0])
	assert.Equal(t, int64(2), labelIDs[1])
	assert.NoError(t, err)
}

func TestGetLabelInRepoByID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	label, err := GetLabelInRepoByID(1, 1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, label.ID)

	_, err = GetLabelInRepoByID(1, -1)
	assert.True(t, IsErrLabelNotExist(err))

	_, err = GetLabelInRepoByID(NonexistentID, NonexistentID)
	assert.True(t, IsErrLabelNotExist(err))
}

func TestGetLabelsInRepoByIDs(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	labels, err := GetLabelsInRepoByIDs(1, []int64{1, 2, NonexistentID})
	assert.NoError(t, err)
	if assert.Len(t, labels, 2) {
		assert.EqualValues(t, 1, labels[0].ID)
		assert.EqualValues(t, 2, labels[1].ID)
	}
}

func TestGetLabelsByRepoID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	testSuccess := func(repoID int64, sortType string, expectedIssueIDs []int64) {
		labels, err := GetLabelsByRepoID(repoID, sortType)
		assert.NoError(t, err)
		assert.Len(t, labels, len(expectedIssueIDs))
		for i, label := range labels {
			assert.EqualValues(t, expectedIssueIDs[i], label.ID)
		}
	}
	testSuccess(1, "leastissues", []int64{2, 1})
	testSuccess(1, "mostissues", []int64{1, 2})
	testSuccess(1, "reversealphabetically", []int64{2, 1})
	testSuccess(1, "default", []int64{1, 2})
}

func TestGetLabelsByIssueID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	labels, err := GetLabelsByIssueID(1)
	assert.NoError(t, err)
	if assert.Len(t, labels, 1) {
		assert.EqualValues(t, 1, labels[0].ID)
	}

	labels, err = GetLabelsByIssueID(NonexistentID)
	assert.NoError(t, err)
	assert.Len(t, labels, 0)
}

func TestUpdateLabel(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	label := AssertExistsAndLoadBean(t, &Label{ID: 1}).(*Label)
	label.Color = "#ffff00"
	label.Name = "newLabelName"
	assert.NoError(t, UpdateLabel(label))
	newLabel := AssertExistsAndLoadBean(t, &Label{ID: 1}).(*Label)
	assert.Equal(t, *label, *newLabel)
	CheckConsistencyFor(t, &Label{}, &Repository{})
}

func TestDeleteLabel(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	label := AssertExistsAndLoadBean(t, &Label{ID: 1}).(*Label)
	assert.NoError(t, DeleteLabel(label.RepoID, label.ID))
	AssertNotExistsBean(t, &Label{ID: label.ID, RepoID: label.RepoID})

	assert.NoError(t, DeleteLabel(label.RepoID, label.ID))
	AssertNotExistsBean(t, &Label{ID: label.ID, RepoID: label.RepoID})

	assert.NoError(t, DeleteLabel(NonexistentID, NonexistentID))
	CheckConsistencyFor(t, &Label{}, &Repository{})
}

func TestHasIssueLabel(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	assert.True(t, HasIssueLabel(1, 1))
	assert.False(t, HasIssueLabel(1, 2))
	assert.False(t, HasIssueLabel(NonexistentID, NonexistentID))
}

func TestNewIssueLabel(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	label := AssertExistsAndLoadBean(t, &Label{ID: 2}).(*Label)
	issue := AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)
	doer := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)

	// add new IssueLabel
	prevNumIssues := label.NumIssues
	assert.NoError(t, NewIssueLabel(issue, label, doer))
	AssertExistsAndLoadBean(t, &IssueLabel{IssueID: issue.ID, LabelID: label.ID})
	AssertExistsAndLoadBean(t, &Comment{
		Type:     CommentTypeLabel,
		PosterID: doer.ID,
		IssueID:  issue.ID,
		LabelID:  label.ID,
		Content:  "1",
	})
	label = AssertExistsAndLoadBean(t, &Label{ID: 2}).(*Label)
	assert.EqualValues(t, prevNumIssues+1, label.NumIssues)

	// re-add existing IssueLabel
	assert.NoError(t, NewIssueLabel(issue, label, doer))
	CheckConsistencyFor(t, &Issue{}, &Label{})
}

func TestNewIssueLabels(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	label1 := AssertExistsAndLoadBean(t, &Label{ID: 1}).(*Label)
	label2 := AssertExistsAndLoadBean(t, &Label{ID: 2}).(*Label)
	issue := AssertExistsAndLoadBean(t, &Issue{ID: 5}).(*Issue)
	doer := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)

	assert.NoError(t, NewIssueLabels(issue, []*Label{label1, label2}, doer))
	AssertExistsAndLoadBean(t, &IssueLabel{IssueID: issue.ID, LabelID: label1.ID})
	AssertExistsAndLoadBean(t, &Comment{
		Type:     CommentTypeLabel,
		PosterID: doer.ID,
		IssueID:  issue.ID,
		LabelID:  label1.ID,
		Content:  "1",
	})
	AssertExistsAndLoadBean(t, &IssueLabel{IssueID: issue.ID, LabelID: label1.ID})
	label1 = AssertExistsAndLoadBean(t, &Label{ID: 1}).(*Label)
	assert.EqualValues(t, 3, label1.NumIssues)
	assert.EqualValues(t, 1, label1.NumClosedIssues)
	label2 = AssertExistsAndLoadBean(t, &Label{ID: 2}).(*Label)
	assert.EqualValues(t, 1, label2.NumIssues)
	assert.EqualValues(t, 1, label2.NumClosedIssues)

	// corner case: test empty slice
	assert.NoError(t, NewIssueLabels(issue, []*Label{}, doer))

	CheckConsistencyFor(t, &Issue{}, &Label{})
}

func TestDeleteIssueLabel(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	testSuccess := func(labelID, issueID, doerID int64) {
		label := AssertExistsAndLoadBean(t, &Label{ID: labelID}).(*Label)
		issue := AssertExistsAndLoadBean(t, &Issue{ID: issueID}).(*Issue)
		doer := AssertExistsAndLoadBean(t, &User{ID: doerID}).(*User)

		expectedNumIssues := label.NumIssues
		expectedNumClosedIssues := label.NumClosedIssues
		if BeanExists(t, &IssueLabel{IssueID: issueID, LabelID: labelID}) {
			expectedNumIssues--
			if issue.IsClosed {
				expectedNumClosedIssues--
			}
		}

		assert.NoError(t, DeleteIssueLabel(issue, label, doer))
		AssertNotExistsBean(t, &IssueLabel{IssueID: issueID, LabelID: labelID})
		AssertExistsAndLoadBean(t, &Comment{
			Type:     CommentTypeLabel,
			PosterID: doerID,
			IssueID:  issueID,
			LabelID:  labelID,
		}, `content=""`)
		label = AssertExistsAndLoadBean(t, &Label{ID: labelID}).(*Label)
		assert.EqualValues(t, expectedNumIssues, label.NumIssues)
		assert.EqualValues(t, expectedNumClosedIssues, label.NumClosedIssues)
	}
	testSuccess(1, 1, 2)
	testSuccess(2, 5, 2)
	testSuccess(1, 1, 2) // delete non-existent IssueLabel

	CheckConsistencyFor(t, &Issue{}, &Label{})
}
