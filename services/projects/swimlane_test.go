// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildLabelSwimlanes(t *testing.T) {
	columns := []*project_model.Column{{ID: 1}, {ID: 2}, {ID: 3}}
	labelA := &issues_model.Label{ID: 10, Name: "Label A"}
	labelB := &issues_model.Label{ID: 20, Name: "label B"}
	otherLabelA := &issues_model.Label{ID: 30, Name: "Label A"}
	multi := &issues_model.Issue{ID: 1, Labels: []*issues_model.Label{labelB, labelA}}
	unlabeled := &issues_model.Issue{ID: 2}
	done := &issues_model.Issue{ID: 3, IsClosed: true, Labels: []*issues_model.Label{labelA}}
	other := &issues_model.Issue{ID: 4, Labels: []*issues_model.Label{otherLabelA}}
	issues := map[int64]issues_model.IssueList{1: {multi, unlabeled, other}, 2: {done}}
	lanes, counts := BuildLabelSwimlanes(columns, issues, nil)
	require.Len(t, lanes, 4)
	assert.Equal(t, []int64{10, 30, 20, 0}, []int64{lanes[0].LabelID, lanes[1].LabelID, lanes[2].LabelID, lanes[3].LabelID})
	assert.Equal(t, issues_model.IssueList{multi}, lanes[0].Issues[1])
	assert.Equal(t, issues_model.IssueList{done}, lanes[0].Issues[2])
	assert.Empty(t, lanes[0].Issues[3])
	assert.Equal(t, 2, lanes[0].NumIssues)
	assert.Equal(t, issues_model.IssueList{multi}, lanes[2].Issues[1])
	assert.Equal(t, issues_model.IssueList{unlabeled}, lanes[3].Issues[1])
	assert.Nil(t, lanes[3].Label)
	assert.Equal(t, map[int64]int64{1: 3, 2: 1}, counts)

	for _, testCase := range []struct {
		name     string
		selected []int64
		laneIDs  []int64
		counts   map[int64]int64
	}{
		{"one row", []int64{10}, []int64{10}, map[int64]int64{1: 1, 2: 1}},
		{"separate rows", []int64{10, 30}, []int64{10, 30}, map[int64]int64{1: 2, 2: 1}},
		{"overlapping rows", []int64{10, 20}, []int64{10, 20}, map[int64]int64{1: 1, 2: 1}},
		{"excluded row", []int64{-10}, []int64{30, 20, 0}, map[int64]int64{1: 3}},
		{"included and excluded rows", []int64{10, -20}, []int64{10}, map[int64]int64{1: 1, 2: 1}},
		{"no label", []int64{0}, []int64{0}, map[int64]int64{1: 1}},
		{"unknown label", []int64{99}, []int64{}, map[int64]int64{}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			selectedLanes, selectedCounts := BuildLabelSwimlanes(columns, issues, testCase.selected)
			laneIDs := make([]int64, 0, len(selectedLanes))
			for _, lane := range selectedLanes {
				laneIDs = append(laneIDs, lane.LabelID)
			}
			assert.Equal(t, testCase.laneIDs, laneIDs)
			assert.Equal(t, testCase.counts, selectedCounts)
		})
	}
	assert.Equal(t, issues_model.IssueList{multi, unlabeled, other}, issues[1])
	assert.Equal(t, []*issues_model.Label{labelB, labelA}, multi.Labels)
	lanes, counts = BuildLabelSwimlanes(columns, nil, nil)
	assert.Empty(t, lanes)
	assert.Empty(t, counts)
}
