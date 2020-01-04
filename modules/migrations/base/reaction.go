// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

// Reaction represents a reaction to an issue/pr/comment.
type Reaction struct {
	UserID   int64
	UserName string
	Content  string
}
