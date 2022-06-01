// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

const (
	pin   = true
	unpin = false
)

func TestUserPinUnpin(t *testing.T) {
	unittest.PrepareTestEnv(t)
	// These test cases run sequentially since they modify state
	testcases := []struct {
		uid          int64
		rid          int64
		action       bool
		endstate     bool
		failmesssage string
	}{
		{
			uid:          2,
			rid:          2,
			action:       pin,
			endstate:     pin,
			failmesssage: "user cannot pin repos they own",
		},
		{
			uid:          2,
			rid:          2,
			action:       unpin,
			endstate:     unpin,
			failmesssage: "user cannot unpin repos they own",
		},

		{
			uid:          2,
			rid:          5,
			action:       pin,
			endstate:     pin,
			failmesssage: "user cannot pin repos they have admin access to",
		},
		{
			uid:          2,
			rid:          5,
			action:       unpin,
			endstate:     unpin,
			failmesssage: "user cannot unpin repos they have admin access to",
		},

		{
			uid:          2,
			rid:          4,
			action:       pin,
			endstate:     unpin,
			failmesssage: "user can pin repos they don't have access to",
		},

		{
			uid:          5,
			rid:          4,
			action:       pin,
			endstate:     pin,
			failmesssage: "user cannot pin repos they own (this should never fail)",
		},
		{
			uid:          2,
			rid:          4,
			action:       unpin,
			endstate:     pin,
			failmesssage: "user can unpin repos they don't have access to",
		},
		{
			uid:          1,
			rid:          4,
			action:       unpin,
			endstate:     unpin,
			failmesssage: "admin can't unpin repos they don't have access to",
		},
		{
			uid:          1,
			rid:          4,
			action:       pin,
			endstate:     pin,
			failmesssage: "admin can't pin repos they don't have access to",
		},
	}

	for _, c := range testcases {
		ctx := test.MockContext(t, "")
		test.LoadUser(t, ctx, c.uid)
		test.LoadRepo(t, ctx, c.rid)

		switch c.action {
		case pin:
			ctx.SetParams(":action", "pin")
		case unpin:
			ctx.SetParams(":action", "unpin")
		}

		Action(ctx)
		ispinned := getRepository(ctx, c.rid).IsPinned()

		assert.Equal(t, ispinned, c.endstate, c.failmesssage)

		if c.endstate != ispinned {
			// We have to stop at first failure, state won't be coherent afterwards.
			return
		}
	}
}
