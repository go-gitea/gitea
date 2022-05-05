// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migration

import "time"

// Milestone defines a standard milestone
type Milestone struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Deadline    *time.Time `json:"deadline"`
	Created     time.Time  `json:"created"`
	Updated     *time.Time `json:"updated"`
	Closed      *time.Time `json:"closed"`
	State       string     `json:"state"` // open, closed
}
