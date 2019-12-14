// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

// Reactions represents a summary of reactions.
type Reactions struct {
	TotalCount int
	PlusOne    int
	MinusOne   int
	Laugh      int
	Confused   int
	Heart      int
	Hooray     int
}
