// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogs

type Team struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Permission  string `json:"permission"`
}

type CreateTeamOption struct {
	Name        string `json:"name" binding:"Required;AlphaDashDot;MaxSize(30)"`
	Description string `json:"description" binding:"MaxSize(255)"`
	Permission  string `json:"permission"`
}
