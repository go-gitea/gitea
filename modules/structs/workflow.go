// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

type Workflow struct {
	BadgeURL  string `json:"badge_url"`
	CreatedAt string `json:"created_at"`
	HTMLURL   string `json:"html_url"`
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	NodeID    string `json:"node_id"`
	Path      string `json:"path"`
	State     string `json:"state"`
	UpdatedAt string `json:"updated_at"`
	URL       string `json:"url"`
}

type WorkflowRun struct {
	Actor *User `json:"actor"`
	
}
