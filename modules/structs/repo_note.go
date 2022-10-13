// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// Note contains information related to a git note
type Note struct {
	Message string  `json:"message"`
	Commit  *Commit `json:"commit"`
}
