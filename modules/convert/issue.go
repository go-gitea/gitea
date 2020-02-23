// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"strings"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
)

// ToTrackedTime converts TrackedTime to API format
func ToTrackedTime(t *models.TrackedTime) (apiT *api.TrackedTime) {
	apiT = &api.TrackedTime{
		ID:       t.ID,
		IssueID:  t.IssueID,
		UserID:   t.UserID,
		UserName: t.User.Name,
		Time:     t.Time,
		Created:  t.Created,
	}
	if t.Issue != nil {
		apiT.Issue = t.Issue.APIFormat()
	}
	if t.User != nil {
		apiT.UserName = t.User.Name
	}
	return
}

// ToTrackedTimeList converts TrackedTimeList to API format
func ToTrackedTimeList(tl models.TrackedTimeList) api.TrackedTimeList {
	result := make([]*api.TrackedTime, 0, len(tl))
	for _, t := range tl {
		result = append(result, ToTrackedTime(t))
	}
	return result
}

// ToLabel converts Label to API format
func ToLabel(label *models.Label) (apiT *api.Label) {
	return &api.Label{
		ID:          label.ID,
		Name:        label.Name,
		Color:       strings.TrimLeft(label.Color, "#"),
		Description: label.Description,
	}
}

// ToLabelList converts list of Label to API format
func ToLabelList(labels []*models.Label) (apiT []*api.Label) {
	result := make([]*api.Label, len(labels))
	for i := range labels {
		result[i] = ToLabel(labels[i])
	}
	return result
}
