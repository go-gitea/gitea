// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestPaginator(t *testing.T) {
	cases := []struct {
		Paginator
		Skip  int
		Take  int
		Start int
		End   int
	}{
		{
			Paginator: &ListOptions{Page: -1, PageSize: -1},
			Skip:      0,
			Take:      setting.API.DefaultPagingNum,
			Start:     0,
			End:       setting.API.DefaultPagingNum,
		},
		{
			Paginator: &ListOptions{Page: 2, PageSize: 10},
			Skip:      10,
			Take:      10,
			Start:     10,
			End:       20,
		},
		{
			Paginator: NewAbsoluteListOptions(-1, -1),
			Skip:      0,
			Take:      setting.API.DefaultPagingNum,
			Start:     0,
			End:       setting.API.DefaultPagingNum,
		},
		{
			Paginator: NewAbsoluteListOptions(2, 10),
			Skip:      2,
			Take:      10,
			Start:     2,
			End:       12,
		},
	}

	for _, c := range cases {
		skip, take := c.Paginator.GetSkipTake()
		start, end := c.Paginator.GetStartEnd()

		assert.Equal(t, c.Skip, skip)
		assert.Equal(t, c.Take, take)
		assert.Equal(t, c.Start, start)
		assert.Equal(t, c.End, end)
	}
}
