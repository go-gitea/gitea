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

	"github.com/stretchr/testify/assert"
)

func TestLabel_CalOpenIssues(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})
	label.CalOpenIssues()
	assert.EqualValues(t, 2, label.NumOpenIssues)
}

func TestLabel_ExclusiveScope(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 7})
	assert.Equal(t, "scope", label.ExclusiveScope())

	label = unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 9})
	assert.Equal(t, "scope/subscope", label.ExclusiveScope())
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
	for _, label := range labels {
		unittest.AssertNotExistsBean(t, label)
	}
	assert.NoError(t, issues_model.NewLabels(labels...))
	for _, label := range labels {
		unittest.AssertExistsAndLoadBean(t, label, unittest.Cond("id = ?", label.ID))
	}
	unittest.CheckConsistencyFor(t, &issues_model.Label{}, &repo_model.Repository{})
}

func TestGetLabelByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	label, err := issues_model.GetLabelByID(db.DefaultContext, 1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, label.ID)

	_, err = issues_model.GetLabelByID(db.DefaultContext, unittest.NonexistentID)
	assert.True(t, issues_model.IsErrLabelNotExist(err))
}

func TestGetLabelInRepoByName(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	label, err := issues_model.GetLabelInRepoByName(db.DefaultContext, 1, "label1")
	assert.NoError(t, err)
	assert.EqualValues(t, 1, label.ID)
	assert.Equal(t, "label1", label.Name)

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
	label, err := issues_model.GetLabelInRepoByID(db.DefaultContext, 1, 1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, label.ID)

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
		for i, label := range labels {
			assert.EqualValues(t, expectedIssueIDs[i], label.ID)
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
	label, err := issues_model.GetLabelInOrgByName(db.DefaultContext, 3, "orglabel3")
	assert.NoError(t, err)
	assert.EqualValues(t, 3, label.ID)
	assert.Equal(t, "orglabel3", label.Name)

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
	label, err := issues_model.GetLabelInOrgByID(db.DefaultContext, 3, 3)
	assert.NoError(t, err)
	assert.EqualValues(t, 3, label.ID)

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
		for i, label := range labels {
			assert.EqualValues(t, expectedIssueIDs[i], label.ID)
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
	label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})
	// make sure update wont overwrite it
	update := &issues_model.Label{
		ID:          label.ID,
		Color:       "#ffff00",
		Name:        "newLabelName",
		Description: label.Description,
		Exclusive:   false,
	}
	label.Color = update.Color
	label.Name = update.Name
	assert.NoError(t, issues_model.UpdateLabel(update))
	newLabel := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})
	assert.EqualValues(t, label.ID, newLabel.ID)
	assert.EqualValues(t, label.Color, newLabel.Color)
	assert.EqualValues(t, label.Name, newLabel.Name)
	assert.EqualValues(t, label.Description, newLabel.Description)
	unittest.CheckConsistencyFor(t, &issues_model.Label{}, &repo_model.Repository{})
}

func TestDeleteLabel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})
	assert.NoError(t, issues_model.DeleteLabel(label.RepoID, label.ID))
	unittest.AssertNotExistsBean(t, &issues_model.Label{ID: label.ID, RepoID: label.RepoID})

	assert.NoError(t, issues_model.DeleteLabel(label.RepoID, label.ID))
	unittest.AssertNotExistsBean(t, &issues_model.Label{ID: label.ID})

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
	label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 2})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// add new IssueLabel
	prevNumIssues := label.NumIssues
	assert.NoError(t, issues_model.NewIssueLabel(issue, label, doer))
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: label.ID})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		Type:     issues_model.CommentTypeLabel,
		PosterID: doer.ID,
		IssueID:  issue.ID,
		LabelID:  label.ID,
		Content:  "1",
	})
	label = unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 2})
	assert.EqualValues(t, prevNumIssues+1, label.NumIssues)

	// re-add existing IssueLabel
	assert.NoError(t, issues_model.NewIssueLabel(issue, label, doer))
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
		label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: labelID})
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issueID})
		doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: doerID})

		expectedNumIssues := label.NumIssues
		expectedNumClosedIssues := label.NumClosedIssues
		if unittest.BeanExists(t, &issues_model.IssueLabel{IssueID: issueID, LabelID: labelID}) {
			expectedNumIssues--
			if issue.IsClosed {
				expectedNumClosedIssues--
			}
		}

		ctx, committer, err := db.TxContext(db.DefaultContext)
		defer committer.Close()
		assert.NoError(t, err)
		assert.NoError(t, issues_model.DeleteIssueLabel(ctx, issue, label, doer))
		assert.NoError(t, committer.Commit())

		unittest.AssertNotExistsBean(t, &issues_model.IssueLabel{IssueID: issueID, LabelID: labelID})
		unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
			Type:     issues_model.CommentTypeLabel,
			PosterID: doerID,
			IssueID:  issueID,
			LabelID:  labelID,
		}, `content=""`)
		label = unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: labelID})
		assert.EqualValues(t, expectedNumIssues, label.NumIssues)
		assert.EqualValues(t, expectedNumClosedIssues, label.NumClosedIssues)
	}
	testSuccess(1, 1, 2)
	testSuccess(2, 5, 2)
	testSuccess(1, 1, 2) // delete non-existent IssueLabel

	unittest.CheckConsistencyFor(t, &issues_model.Issue{}, &issues_model.Label{})
}
