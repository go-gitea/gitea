// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addCustomWebhooks(x *xorm.Engine) error {
	type HookContentType int

	type HookEvents struct {
		Create               bool `json:"create"`
		Delete               bool `json:"delete"`
		Fork                 bool `json:"fork"`
		Issues               bool `json:"issues"`
		IssueAssign          bool `json:"issue_assign"`
		IssueLabel           bool `json:"issue_label"`
		IssueMilestone       bool `json:"issue_milestone"`
		IssueComment         bool `json:"issue_comment"`
		Push                 bool `json:"push"`
		PullRequest          bool `json:"pull_request"`
		PullRequestAssign    bool `json:"pull_request_assign"`
		PullRequestLabel     bool `json:"pull_request_label"`
		PullRequestMilestone bool `json:"pull_request_milestone"`
		PullRequestComment   bool `json:"pull_request_comment"`
		PullRequestReview    bool `json:"pull_request_review"`
		PullRequestSync      bool `json:"pull_request_sync"`
		Repository           bool `json:"repository"`
		Release              bool `json:"release"`
	}

	type HookEvent struct {
		PushOnly       bool   `json:"push_only"`
		SendEverything bool   `json:"send_everything"`
		ChooseEvents   bool   `json:"choose_events"`
		BranchFilter   string `json:"branch_filter"`

		HookEvents `json:"events"`
	}

	type HookType = string

	type HookStatus int

	type Webhook struct {
		ID              int64  `xorm:"pk autoincr"`
		CustomID        string `xorm:"VARCHAR(20) 'custom_id'"`
		RepoID          int64  `xorm:"INDEX"` // An ID of 0 indicates either a default or system webhook
		OrgID           int64  `xorm:"INDEX"`
		IsSystemWebhook bool
		URL             string `xorm:"url TEXT"`
		HTTPMethod      string `xorm:"http_method"`
		ContentType     HookContentType
		Secret          string `xorm:"TEXT"`
		Events          string `xorm:"TEXT"`
		*HookEvent      `xorm:"-"`
		IsActive        bool       `xorm:"INDEX"`
		Type            HookType   `xorm:"VARCHAR(16) 'type'"`
		Meta            string     `xorm:"TEXT"` // store hook-specific attributes
		LastStatus      HookStatus // Last delivery status

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	if err := x.Sync2(new(Webhook)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
