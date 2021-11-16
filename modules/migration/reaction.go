// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migration

// Reaction represents a reaction to an issue/pr/comment.
type Reaction struct {
	UserID   int64  `yaml:"user_id"`
	UserName string `yaml:"user_name"`
	Content  string
}
