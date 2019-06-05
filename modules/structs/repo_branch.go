// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// Branch represents a repository branch
type Branch struct {
	Name   string         `json:"name"`
	Commit *PayloadCommit `json:"commit"`
}
