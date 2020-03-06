// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"encoding/json"

	"xorm.io/xorm"
)

func expandWebhooks(x *xorm.Engine) error {

	type ChooseEvents struct {
		Issues               bool `json:"issues"`
		IssueAssign          bool `json:"issue_assign"`
		IssueLabel           bool `json:"issue_label"`
		IssueMilestone       bool `json:"issue_milestone"`
		IssueComment         bool `json:"issue_comment"`
		PullRequest          bool `json:"pull_request"`
		PullRequestAssign    bool `json:"pull_request_assign"`
		PullRequestLabel     bool `json:"pull_request_label"`
		PullRequestMilestone bool `json:"pull_request_milestone"`
		PullRequestComment   bool `json:"pull_request_comment"`
		PullRequestReview    bool `json:"pull_request_review"`
		PullRequestSync      bool `json:"pull_request_sync"`
	}

	type Events struct {
		PushOnly       bool         `json:"push_only"`
		SendEverything bool         `json:"send_everything"`
		ChooseEvents   bool         `json:"choose_events"`
		BranchFilter   string       `json:"branch_filter"`
		Events         ChooseEvents `json:"events"`
	}

	type Webhook struct {
		ID     int64
		Events string
	}

	var events Events
	var bytes []byte
	var last int
	const batchSize = 50
	sess := x.NewSession()
	defer sess.Close()
	for {
		if err := sess.Begin(); err != nil {
			return err
		}
		var results = make([]Webhook, 0, batchSize)
		err := x.OrderBy("id").
			Limit(batchSize, last).
			Find(&results)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			break
		}
		last += len(results)

		for _, res := range results {
			if err = json.Unmarshal([]byte(res.Events), &events); err != nil {
				return err
			}

			if events.Events.Issues {
				events.Events.IssueAssign = true
				events.Events.IssueLabel = true
				events.Events.IssueMilestone = true
				events.Events.IssueComment = true
			}

			if events.Events.PullRequest {
				events.Events.PullRequestAssign = true
				events.Events.PullRequestLabel = true
				events.Events.PullRequestMilestone = true
				events.Events.PullRequestComment = true
				events.Events.PullRequestReview = true
				events.Events.PullRequestSync = true
			}

			if bytes, err = json.Marshal(&events); err != nil {
				return err
			}

			_, err = sess.Exec("UPDATE webhook SET events = ? WHERE id = ?", string(bytes), res.ID)
			if err != nil {
				return err
			}
		}

		if err := sess.Commit(); err != nil {
			return err
		}
	}
	return nil
}
