// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

type HookEvents map[HookEventType]bool

func (he HookEvents) Get(evt HookEventType) bool {
	return he[evt]
}

// HookEvent represents events that will delivery hook.
type HookEvent struct {
	PushOnly       bool   `json:"push_only"`
	SendEverything bool   `json:"send_everything"`
	ChooseEvents   bool   `json:"choose_events"`
	BranchFilter   string `json:"branch_filter"`

	HookEvents `json:"events"`
}
