// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import "time"

// Milestone defines a standard milestone
type Milestone struct {
	Title       string
	Description string
	Deadline    *time.Time
	Created     time.Time
	Updated     *time.Time
	Closed      *time.Time
	State       string
}
