// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/contexttest"
	"code.gitea.io/gitea/services/forms"

	"github.com/stretchr/testify/assert"
)

func TestIssueLabel(t *testing.T) {
	unittest.PrepareTestEnv(t)
	t.Run("RetrieveLabels", testRetrieveLabels)
	t.Run("NewLabel", testNewLabel)
	t.Run("NewLabelInvalidColor", testNewLabelInvalidColor)
	t.Run("UpdateLabel", testUpdateLabel)
	t.Run("UpdateLabelInvalidColor", testUpdateLabelInvalidColor)
	t.Run("UpdateIssueLabelClear", testUpdateIssueLabelClear)
	t.Run("UpdateIssueLabelToggle", testUpdateIssueLabelToggle)
	t.Run("InitializeLabels", testInitializeLabels)
	t.Run("DeleteLabel", testDeleteLabel)
}

func testInitializeLabels(t *testing.T) {
	assert.NoError(t, repository.LoadRepoConfig())
	ctx, _ := contexttest.MockContext(t, "user2/repo1/labels/initialize")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 2)
	web.SetForm(ctx, &forms.InitializeLabelsForm{TemplateName: "Default"})
	InitializeLabels(ctx)
	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	unittest.AssertExistsAndLoadBean(t, &issues_model.Label{
		RepoID: 2,
		Name:   "enhancement",
		Color:  "#84b6eb",
	})
	assert.Equal(t, "/user2/repo2/labels", test.RedirectURL(ctx.Resp))
}

func testRetrieveLabels(t *testing.T) {
	for _, testCase := range []struct {
		RepoID           int64
		Sort             string
		ExpectedLabelIDs []int64
	}{
		{1, "", []int64{1, 2}},
		{1, "leastissues", []int64{2, 1}},
		{2, "", []int64{}},
	} {
		ctx, _ := contexttest.MockContext(t, "user/repo/issues")
		contexttest.LoadUser(t, ctx, 2)
		contexttest.LoadRepo(t, ctx, testCase.RepoID)
		ctx.Req.Form.Set("sort", testCase.Sort)
		RetrieveLabelsForList(ctx)
		assert.False(t, ctx.Written())
		labels, ok := ctx.Data["Labels"].([]*issues_model.Label)
		assert.True(t, ok)
		if assert.Len(t, labels, len(testCase.ExpectedLabelIDs)) {
			for i, label := range labels {
				assert.Equal(t, testCase.ExpectedLabelIDs[i], label.ID)
			}
		}
	}
}

func testNewLabel(t *testing.T) {
	ctx, respWriter := contexttest.MockContext(t, "user2/repo1/labels/edit")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	web.SetForm(ctx, &forms.CreateLabelForm{
		Title: "newlabel",
		Color: "#abcdef",
	})
	NewLabel(ctx)
	assert.Equal(t, http.StatusOK, ctx.Resp.WrittenStatus())
	unittest.AssertExistsAndLoadBean(t, &issues_model.Label{
		Name:  "newlabel",
		Color: "#abcdef",
	})
	assert.Equal(t, "/user2/repo1/labels", test.RedirectURL(respWriter))
}

func testNewLabelInvalidColor(t *testing.T) {
	ctx, respWriter := contexttest.MockContext(t, "user2/repo1/labels/edit")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	web.SetForm(ctx, &forms.CreateLabelForm{
		Title: "newlabel-x",
		Color: "bad-label-code",
	})
	NewLabel(ctx)
	assert.Equal(t, http.StatusBadRequest, ctx.Resp.WrittenStatus())
	assert.Equal(t, "repo.issues.label_color_invalid", test.ParseJSONError(respWriter.Body.Bytes()).ErrorMessage)
	unittest.AssertNotExistsBean(t, &issues_model.Label{
		Name: "newlabel-x",
	})
}

func testUpdateLabel(t *testing.T) {
	ctx, respWriter := contexttest.MockContext(t, "user2/repo1/labels/edit")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	web.SetForm(ctx, &forms.CreateLabelForm{
		ID:         2,
		Title:      "newnameforlabel",
		Color:      "#abcdef",
		IsArchived: true,
	})
	UpdateLabel(ctx)
	assert.Equal(t, http.StatusOK, ctx.Resp.WrittenStatus())
	unittest.AssertExistsAndLoadBean(t, &issues_model.Label{
		ID:    2,
		Name:  "newnameforlabel",
		Color: "#abcdef",
	})
	assert.Equal(t, "/user2/repo1/labels", test.RedirectURL(respWriter))
}

func testUpdateLabelInvalidColor(t *testing.T) {
	ctx, respWriter := contexttest.MockContext(t, "user2/repo1/labels/edit")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	web.SetForm(ctx, &forms.CreateLabelForm{
		ID:    1,
		Title: "label1",
		Color: "bad-label-code",
	})

	UpdateLabel(ctx)

	assert.Equal(t, http.StatusBadRequest, ctx.Resp.WrittenStatus())
	assert.Equal(t, "repo.issues.label_color_invalid", test.ParseJSONError(respWriter.Body.Bytes()).ErrorMessage)
	unittest.AssertExistsAndLoadBean(t, &issues_model.Label{
		ID:    1,
		Name:  "label1",
		Color: "#abcdef",
	})
}

func testDeleteLabel(t *testing.T) {
	ctx, _ := contexttest.MockContext(t, "user2/repo1/labels/delete")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	ctx.Req.Form.Set("id", "2")
	DeleteLabel(ctx)
	assert.Equal(t, http.StatusOK, ctx.Resp.WrittenStatus())
	unittest.AssertNotExistsBean(t, &issues_model.Label{ID: 2})
	unittest.AssertNotExistsBean(t, &issues_model.IssueLabel{LabelID: 2})
	assert.EqualValues(t, ctx.Tr("repo.issues.label_deletion_success"), ctx.Flash.SuccessMsg)
}

func testUpdateIssueLabelClear(t *testing.T) {
	ctx, _ := contexttest.MockContext(t, "user2/repo1/issues/labels")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	ctx.Req.Form.Set("issue_ids", "1,3")
	ctx.Req.Form.Set("action", "clear")
	UpdateIssueLabel(ctx)
	assert.Equal(t, http.StatusOK, ctx.Resp.WrittenStatus())
	unittest.AssertNotExistsBean(t, &issues_model.IssueLabel{IssueID: 1})
	unittest.AssertNotExistsBean(t, &issues_model.IssueLabel{IssueID: 3})
	unittest.CheckConsistencyFor(t, &issues_model.Label{})
}

func testUpdateIssueLabelToggle(t *testing.T) {
	for _, testCase := range []struct {
		Action      string
		IssueIDs    []int64
		LabelID     int64
		ExpectedAdd bool // whether we expect the label to be added to the issues
	}{
		{"attach", []int64{1, 3}, 1, true},
		{"detach", []int64{1, 3}, 1, false},
		{"toggle", []int64{1, 3}, 1, false},
		{"toggle", []int64{1, 2}, 2, true},
	} {
		unittest.PrepareTestEnv(t)
		ctx, _ := contexttest.MockContext(t, "user2/repo1/issues/labels")
		contexttest.LoadUser(t, ctx, 2)
		contexttest.LoadRepo(t, ctx, 1)

		ctx.Req.Form.Set("issue_ids", strings.Join(base.Int64sToStrings(testCase.IssueIDs), ","))
		ctx.Req.Form.Set("action", testCase.Action)
		ctx.Req.Form.Set("id", strconv.Itoa(int(testCase.LabelID)))
		UpdateIssueLabel(ctx)
		assert.Equal(t, http.StatusOK, ctx.Resp.WrittenStatus())
		for _, issueID := range testCase.IssueIDs {
			if testCase.ExpectedAdd {
				unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issueID, LabelID: testCase.LabelID})
			} else {
				unittest.AssertNotExistsBean(t, &issues_model.IssueLabel{IssueID: issueID, LabelID: testCase.LabelID})
			}
		}
		unittest.CheckConsistencyFor(t, &issues_model.Label{})
	}
}
