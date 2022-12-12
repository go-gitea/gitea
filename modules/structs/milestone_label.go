// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// MilestoneLabelsOption a collection of labels
type MilestoneLabelsOption struct {
	// list of label IDs
	Labels []int64 `json:"labels"`
}
