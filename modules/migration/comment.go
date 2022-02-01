// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migration

import "time"

// Comment is a standard comment information
type Comment struct {
	IssueIndex  int64       `yaml:"issue_index" json:"issue_index"`
	PosterID    int64       `yaml:"poster_id" json:"poster_id"`
	PosterName  string      `yaml:"poster_name" json:"poster_name"`
	PosterEmail string      `yaml:"poster_email" json:"poster_email"`
	Created     time.Time   `json:"created"`
	Updated     time.Time   `json:"updated"`
	Content     string      `json:"content"`
	Reactions   []*Reaction `json:"reactions"`
}
