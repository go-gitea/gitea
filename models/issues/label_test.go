// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/label"

	"github.com/stretchr/testify/assert"
)

func TestLabel_CalOpenIssues(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	l := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})
	l.CalOpenIssues()
	assert.EqualValues(t, 2, l.NumOpenIssues)
}

func TestLabel_TextColor(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	l := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})
	assert.False(t, l.UseLightTextColor())

	l = unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 2})
	assert.True(t, l.UseLightTextColor())
}

func TestLabel_ExclusiveScope(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	l := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 7})
	assert.Equal(t, "scope", l.ExclusiveScope())

	l = unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 9})
	assert.Equal(t, "scope/subscope", l.ExclusiveScope())
}

func TestNewLabels(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	labels := []*issues_model.Label{
		{RepoID: 2, Name: "labelName2", Color: "#123456"},
		{RepoID: 3, Name: "labelName3", Color: "#123"},
		{RepoID: 4, Name: "labelName4", Color: "ABCDEF"},
		{RepoID: 5, Name: "labelName5", Color: "DEF"},
	}
	assert.Error(t, issues_model.NewLabel(db.DefaultContext, &issues_model.Label{RepoID: 3, Name: "invalid Color", Color: ""}))
	assert.Error(t, issues_model.NewLabel(db.DefaultContext, &issues_model.Label{RepoID: 3, Name: "invalid Color", Color: "#45G"}))
	assert.Error(t, issues_model.NewLabel(db.DefaultContext, &issues_model.Label{RepoID: 3, Name: "invalid Color", Color: "#12345G"}))
	assert.Error(t, issues_model.NewLabel(db.DefaultContext, &issues_model.Label{RepoID: 3, Name: "invalid Color", Color: "45G"}))
	assert.Error(t, issues_model.NewLabel(db.DefaultContext, &issues_model.Label{RepoID: 3, Name: "invalid Color", Color: "12345G"}))
	for _, l := range labels {
		unittest.AssertNotExistsBean(t, l)
	}
	assert.NoError(t, issues_model.NewLabels(labels...))
	for _, l := range labels {
		unittest.AssertExistsAndLoadBean(t, l, unittest.Cond("id = ?", l.ID))
	}
	unittest.CheckConsistencyFor(t, &issues_model.Label{}, &repo_model.Repository{})
}

func TestGetLabelByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	l, err := issues_model.GetLabelByID(db.DefaultContext, 1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, l.ID)

	_, err = issues_model.GetLabelByID(db.DefaultContext, unittest.NonexistentID)
	assert.True(t, issues_model.IsErrLabelNotExist(err))
}

func TestGetLabelInRepoByName(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	l, err := issues_model.GetLabelInRepoByName(db.DefaultContext, 1, "label1")
	assert.NoError(t, err)
	assert.EqualValues(t, 1, l.ID)
	assert.Equal(t, "label1", l.Name)

	_, err = issues_model.GetLabelInRepoByName(db.DefaultContext, 1, "")
	assert.True(t, issues_model.IsErrRepoLabelNotExist(err))

	_, err = issues_model.GetLabelInRepoByName(db.DefaultContext, unittest.NonexistentID, "nonexistent")
	assert.True(t, issues_model.IsErrRepoLabelNotExist(err))
}

func TestGetLabelInRepoByNames(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	labelIDs, err := issues_model.GetLabelIDsInRepoByNames(1, []string{"label1", "label2"})
	assert.NoError(t, err)

	assert.Len(t, labelIDs, 2)

	assert.Equal(t, int64(1), labelIDs[0])
	assert.Equal(t, int64(2), labelIDs[1])
}

func TestGetLabelInRepoByNamesDiscardsNonExistentLabels(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	// label3 doesn't exists.. See labels.yml
	labelIDs, err := issues_model.GetLabelIDsInRepoByNames(1, []string{"label1", "label2", "label3"})
	assert.NoError(t, err)

	assert.Len(t, labelIDs, 2)

	assert.Equal(t, int64(1), labelIDs[0])
	assert.Equal(t, int64(2), labelIDs[1])
	assert.NoError(t, err)
}

func TestGetLabelInRepoByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	l, err := issues_model.GetLabelInRepoByID(db.DefaultContext, 1, 1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, l.ID)

	_, err = issues_model.GetLabelInRepoByID(db.DefaultContext, 1, -1)
	assert.True(t, issues_model.IsErrRepoLabelNotExist(err))

	_, err = issues_model.GetLabelInRepoByID(db.DefaultContext, unittest.NonexistentID, unittest.NonexistentID)
	assert.True(t, issues_model.IsErrRepoLabelNotExist(err))
}

func TestGetLabelsInRepoByIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	labels, err := issues_model.GetLabelsInRepoByIDs(db.DefaultContext, 1, []int64{1, 2, unittest.NonexistentID})
	assert.NoError(t, err)
	if assert.Len(t, labels, 2) {
		assert.EqualValues(t, 1, labels[0].ID)
		assert.EqualValues(t, 2, labels[1].ID)
	}
}

func TestGetLabelsByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	testSuccess := func(repoID int64, sortType string, expectedIssueIDs []int64) {
		labels, err := issues_model.GetLabelsByRepoID(db.DefaultContext, repoID, sortType, db.ListOptions{})
		assert.NoError(t, err)
		assert.Len(t, labels, len(expectedIssueIDs))
		for i, l := range labels {
			assert.EqualValues(t, expectedIssueIDs[i], l.ID)
		}
	}
	testSuccess(1, "leastissues", []int64{2, 1})
	testSuccess(1, "mostissues", []int64{1, 2})
	testSuccess(1, "reversealphabetically", []int64{2, 1})
	testSuccess(1, "default", []int64{1, 2})
}

// Org versions

func TestGetLabelInOrgByName(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	l, err := issues_model.GetLabelInOrgByName(db.DefaultContext, 3, "orglabel3")
	assert.NoError(t, err)
	assert.EqualValues(t, 3, l.ID)
	assert.Equal(t, "orglabel3", l.Name)

	_, err = issues_model.GetLabelInOrgByName(db.DefaultContext, 3, "")
	assert.True(t, issues_model.IsErrOrgLabelNotExist(err))

	_, err = issues_model.GetLabelInOrgByName(db.DefaultContext, 0, "orglabel3")
	assert.True(t, issues_model.IsErrOrgLabelNotExist(err))

	_, err = issues_model.GetLabelInOrgByName(db.DefaultContext, -1, "orglabel3")
	assert.True(t, issues_model.IsErrOrgLabelNotExist(err))

	_, err = issues_model.GetLabelInOrgByName(db.DefaultContext, unittest.NonexistentID, "nonexistent")
	assert.True(t, issues_model.IsErrOrgLabelNotExist(err))
}

func TestGetLabelInOrgByNames(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	labelIDs, err := issues_model.GetLabelIDsInOrgByNames(3, []string{"orglabel3", "orglabel4"})
	assert.NoError(t, err)

	assert.Len(t, labelIDs, 2)

	assert.Equal(t, int64(3), labelIDs[0])
	assert.Equal(t, int64(4), labelIDs[1])
}

func TestGetLabelInOrgByNamesDiscardsNonExistentLabels(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	// orglabel99 doesn't exists.. See labels.yml
	labelIDs, err := issues_model.GetLabelIDsInOrgByNames(3, []string{"orglabel3", "orglabel4", "orglabel99"})
	assert.NoError(t, err)

	assert.Len(t, labelIDs, 2)

	assert.Equal(t, int64(3), labelIDs[0])
	assert.Equal(t, int64(4), labelIDs[1])
	assert.NoError(t, err)
}

func TestGetLabelInOrgByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	l, err := issues_model.GetLabelInOrgByID(db.DefaultContext, 3, 3)
	assert.NoError(t, err)
	assert.EqualValues(t, 3, l.ID)

	_, err = issues_model.GetLabelInOrgByID(db.DefaultContext, 3, -1)
	assert.True(t, issues_model.IsErrOrgLabelNotExist(err))

	_, err = issues_model.GetLabelInOrgByID(db.DefaultContext, 0, 3)
	assert.True(t, issues_model.IsErrOrgLabelNotExist(err))

	_, err = issues_model.GetLabelInOrgByID(db.DefaultContext, -1, 3)
	assert.True(t, issues_model.IsErrOrgLabelNotExist(err))

	_, err = issues_model.GetLabelInOrgByID(db.DefaultContext, unittest.NonexistentID, unittest.NonexistentID)
	assert.True(t, issues_model.IsErrOrgLabelNotExist(err))
}

func TestGetLabelsInOrgByIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	labels, err := issues_model.GetLabelsInOrgByIDs(db.DefaultContext, 3, []int64{3, 4, unittest.NonexistentID})
	assert.NoError(t, err)
	if assert.Len(t, labels, 2) {
		assert.EqualValues(t, 3, labels[0].ID)
		assert.EqualValues(t, 4, labels[1].ID)
	}
}

func TestGetLabelsByOrgID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	testSuccess := func(orgID int64, sortType string, expectedIssueIDs []int64) {
		labels, err := issues_model.GetLabelsByOrgID(db.DefaultContext, orgID, sortType, db.ListOptions{})
		assert.NoError(t, err)
		assert.Len(t, labels, len(expectedIssueIDs))
		for i, l := range labels {
			assert.EqualValues(t, expectedIssueIDs[i], l.ID)
		}
	}
	testSuccess(3, "leastissues", []int64{3, 4})
	testSuccess(3, "mostissues", []int64{4, 3})
	testSuccess(3, "reversealphabetically", []int64{4, 3})
	testSuccess(3, "default", []int64{3, 4})

	var err error
	_, err = issues_model.GetLabelsByOrgID(db.DefaultContext, 0, "leastissues", db.ListOptions{})
	assert.True(t, issues_model.IsErrOrgLabelNotExist(err))

	_, err = issues_model.GetLabelsByOrgID(db.DefaultContext, -1, "leastissues", db.ListOptions{})
	assert.True(t, issues_model.IsErrOrgLabelNotExist(err))
}

//

func TestGetLabelsByIssueID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	labels, err := issues_model.GetLabelsByIssueID(db.DefaultContext, 1)
	assert.NoError(t, err)
	if assert.Len(t, labels, 1) {
		assert.EqualValues(t, 1, labels[0].ID)
	}

	labels, err = issues_model.GetLabelsByIssueID(db.DefaultContext, unittest.NonexistentID)
	assert.NoError(t, err)
	assert.Len(t, labels, 0)
}

func TestUpdateLabel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	l := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})
	// make sure update wont overwrite it
	update := &issues_model.Label{
		ID:          l.ID,
		Name:        "newLabelName",
		Exclusive:   false,
		Color:       "#ffff00",
		Priority:    label.Priority("high"),
		Description: l.Description,
	}
	l.Color = update.Color
	l.Name = update.Name
	assert.NoError(t, issues_model.UpdateLabel(update))
	newLabel := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})
	assert.EqualValues(t, l.ID, newLabel.ID)
	assert.EqualValues(t, l.Color, newLabel.Color)
	assert.EqualValues(t, l.Priority, newLabel.Priority)
	assert.EqualValues(t, l.Name, newLabel.Name)
	assert.EqualValues(t, l.Description, newLabel.Description)
	unittest.CheckConsistencyFor(t, &issues_model.Label{}, &repo_model.Repository{})
}

func TestDeleteLabel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	l := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})
	assert.NoError(t, issues_model.DeleteLabel(l.RepoID, l.ID))
	unittest.AssertNotExistsBean(t, &issues_model.Label{ID: l.ID, RepoID: l.RepoID})

	assert.NoError(t, issues_model.DeleteLabel(l.RepoID, l.ID))
	unittest.AssertNotExistsBean(t, &issues_model.Label{ID: l.ID})

	assert.NoError(t, issues_model.DeleteLabel(unittest.NonexistentID, unittest.NonexistentID))
	unittest.CheckConsistencyFor(t, &issues_model.Label{}, &repo_model.Repository{})
}

func TestHasIssueLabel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	assert.True(t, issues_model.HasIssueLabel(db.DefaultContext, 1, 1))
	assert.False(t, issues_model.HasIssueLabel(db.DefaultContext, 1, 2))
	assert.False(t, issues_model.HasIssueLabel(db.DefaultContext, unittest.NonexistentID, unittest.NonexistentID))
}

func TestNewIssueLabel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	l := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 2})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// add new IssueLabel
	prevNumIssues := l.NumIssues
	assert.NoError(t, issues_model.NewIssueLabel(issue, l, doer))
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: l.ID})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		Type:     issues_model.CommentTypeLabel,
		PosterID: doer.ID,
		IssueID:  issue.ID,
		LabelID:  l.ID,
		Content:  "1",
	})
	l = unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 2})
	assert.EqualValues(t, prevNumIssues+1, l.NumIssues)

	// re-add existing IssueLabel
	assert.NoError(t, issues_model.NewIssueLabel(issue, l, doer))
	unittest.CheckConsistencyFor(t, &issues_model.Issue{}, &issues_model.Label{})
}

func TestNewIssueExclusiveLabel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 18})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	otherLabel := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 6})
	exclusiveLabelA := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 7})
	exclusiveLabelB := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 8})

	// coexisting regular and exclusive label
	assert.NoError(t, issues_model.NewIssueLabel(issue, otherLabel, doer))
	assert.NoError(t, issues_model.NewIssueLabel(issue, exclusiveLabelA, doer))
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: otherLabel.ID})
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: exclusiveLabelA.ID})

	// exclusive label replaces existing one
	assert.NoError(t, issues_model.NewIssueLabel(issue, exclusiveLabelB, doer))
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: otherLabel.ID})
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: exclusiveLabelB.ID})
	unittest.AssertNotExistsBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: exclusiveLabelA.ID})

	// exclusive label replaces existing one again
	assert.NoError(t, issues_model.NewIssueLabel(issue, exclusiveLabelA, doer))
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: otherLabel.ID})
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: exclusiveLabelA.ID})
	unittest.AssertNotExistsBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: exclusiveLabelB.ID})
}

func TestNewIssueLabels(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	label1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})
	label2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 2})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 5})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	assert.NoError(t, issues_model.NewIssueLabels(issue, []*issues_model.Label{label1, label2}, doer))
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: label1.ID})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		Type:     issues_model.CommentTypeLabel,
		PosterID: doer.ID,
		IssueID:  issue.ID,
		LabelID:  label1.ID,
		Content:  "1",
	})
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: label1.ID})
	label1 = unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})
	assert.EqualValues(t, 3, label1.NumIssues)
	assert.EqualValues(t, 1, label1.NumClosedIssues)
	label2 = unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 2})
	assert.EqualValues(t, 1, label2.NumIssues)
	assert.EqualValues(t, 1, label2.NumClosedIssues)

	// corner case: test empty slice
	assert.NoError(t, issues_model.NewIssueLabels(issue, []*issues_model.Label{}, doer))

	unittest.CheckConsistencyFor(t, &issues_model.Issue{}, &issues_model.Label{})
}

func TestDeleteIssueLabel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	testSuccess := func(labelID, issueID, doerID int64) {
		l := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: labelID})
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issueID})
		doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: doerID})

		expectedNumIssues := l.NumIssues
		expectedNumClosedIssues := l.NumClosedIssues
		if unittest.BeanExists(t, &issues_model.IssueLabel{IssueID: issueID, LabelID: labelID}) {
			expectedNumIssues--
			if issue.IsClosed {
				expectedNumClosedIssues--
			}
		}

		ctx, committer, err := db.TxContext(db.DefaultContext)
		defer committer.Close()
		assert.NoError(t, err)
		assert.NoError(t, issues_model.DeleteIssueLabel(ctx, issue, l, doer))
		assert.NoError(t, committer.Commit())

		unittest.AssertNotExistsBean(t, &issues_model.IssueLabel{IssueID: issueID, LabelID: labelID})
		unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
			Type:     issues_model.CommentTypeLabel,
			PosterID: doerID,
			IssueID:  issueID,
			LabelID:  labelID,
		}, `content=""`)
		l = unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: labelID})
		assert.EqualValues(t, expectedNumIssues, l.NumIssues)
		assert.EqualValues(t, expectedNumClosedIssues, l.NumClosedIssues)
	}
	testSuccess(1, 1, 2)
	testSuccess(2, 5, 2)
	testSuccess(1, 1, 2) // delete non-existent IssueLabel

	unittest.CheckConsistencyFor(t, &issues_model.Issue{}, &issues_model.Label{})
}
