// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import "time"

// Comment is a standard comment information
type Comment struct {
	IssueIndex  int64
	PosterID    int64
	PosterName  string
	PosterEmail string
	Created     time.Time
	Updated     time.Time
	Content     string
	Reactions   *Reactions
}
