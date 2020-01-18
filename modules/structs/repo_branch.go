// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// Branch represents a repository branch
type Branch struct {
	Name                string         `json:"name"`
	Commit              *PayloadCommit `json:"commit"`
	Protected           bool           `json:"protected"`
	RequiredApprovals   int64          `json:"required_approvals"`
	EnableStatusCheck   bool           `json:"enable_status_check"`
	StatusCheckContexts []string       `json:"status_check_contexts"`
	UserCanPush         bool           `json:"user_can_push"`
	UserCanMerge        bool           `json:"user_can_merge"`
}
