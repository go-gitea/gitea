// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// MilestoneLabelsOption a collection of labels
type MilestoneLabelsOption struct {
	// list of label IDs
	Labels []int64 `json:"labels"`
}
