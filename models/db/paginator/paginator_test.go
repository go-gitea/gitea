// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package paginator

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestPaginator(t *testing.T) {
	cases := []struct {
		db.Paginator
		Skip  int
		Take  int
		Start int
		End   int
	}{
		{
			Paginator: &db.ListOptions{Page: -1, PageSize: -1},
			Skip:      0,
			Take:      setting.API.DefaultPagingNum,
			Start:     0,
			End:       setting.API.DefaultPagingNum,
		},
		{
			Paginator: &db.ListOptions{Page: 2, PageSize: 10},
			Skip:      10,
			Take:      10,
			Start:     10,
			End:       20,
		},
		{
			Paginator: db.NewAbsoluteListOptions(-1, -1),
			Skip:      0,
			Take:      setting.API.DefaultPagingNum,
			Start:     0,
			End:       setting.API.DefaultPagingNum,
		},
		{
			Paginator: db.NewAbsoluteListOptions(2, 10),
			Skip:      2,
			Take:      10,
			Start:     2,
			End:       12,
		},
	}

	for _, c := range cases {
		skip, take := c.Paginator.GetSkipTake()

		assert.Equal(t, c.Skip, skip)
		assert.Equal(t, c.Take, take)
	}
}
