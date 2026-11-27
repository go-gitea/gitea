// Copyright 2022 The Gitea Authors.
// Copyright 2015 https://github.com/unknwon. Licensed under the Apache License, Version 2.0
// SPDX-License-Identifier: Apache-2.0

package paginator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPaginator(t *testing.T) {
	t.Run("Basic logics", func(t *testing.T) {
		p := New(0, -1, -1)
		assert.Equal(t, 1, p.PagingNum())
		assert.True(t, p.IsFirst())
		assert.False(t, p.HasPrevious())
		assert.Equal(t, 1, p.Previous())
		assert.False(t, p.HasNext())
		assert.Equal(t, 1, p.Next())
		assert.True(t, p.IsLast())
		assert.Equal(t, 0, p.Total())

		p = New(1, 10, 2)
		assert.Equal(t, 10, p.PagingNum())
		assert.True(t, p.IsFirst())
		assert.False(t, p.HasPrevious())
		assert.False(t, p.HasNext())
		assert.True(t, p.IsLast())

		p = New(10, 10, 1)
		assert.Equal(t, 10, p.PagingNum())
		assert.True(t, p.IsFirst())
		assert.False(t, p.HasPrevious())
		assert.False(t, p.HasNext())
		assert.True(t, p.IsLast())

		p = New(11, 10, 1)
		assert.Equal(t, 10, p.PagingNum())
		assert.True(t, p.IsFirst())
		assert.False(t, p.HasPrevious())
		assert.True(t, p.HasNext())
		assert.Equal(t, 2, p.Next())
		assert.False(t, p.IsLast())

		p = New(11, 10, 2)
		assert.Equal(t, 10, p.PagingNum())
		assert.False(t, p.IsFirst())
		assert.True(t, p.HasPrevious())
		assert.Equal(t, 1, p.Previous())
		assert.False(t, p.HasNext())
		assert.True(t, p.IsLast())

		p = New(20, 10, 2)
		assert.Equal(t, 10, p.PagingNum())
		assert.False(t, p.IsFirst())
		assert.True(t, p.HasPrevious())
		assert.False(t, p.HasNext())
		assert.True(t, p.IsLast())

		p = New(25, 10, 2)
		assert.Equal(t, 10, p.PagingNum())
		assert.False(t, p.IsFirst())
		assert.True(t, p.HasPrevious())
		assert.True(t, p.HasNext())
		assert.False(t, p.IsLast())
	})

	t.Run("Pages", func(t *testing.T) {
		assert.Empty(t, New(0, 10, 1).Pages())
		assert.Equal(t, []int{3}, New(-1, 10, 3).Pages())
		assert.Equal(t, []int{1, 2, 3, 4, 5, 6, 7}, New(70, 10, 4).Pages())
		assert.Equal(t, []int{1, 2, 3, 4, 5, -1, 8}, New(80, 10, 4).Pages())
		assert.Equal(t, []int{1, -1, 4, 5, 6, 7, 8}, New(80, 10, 5).Pages())
		assert.Equal(t, []int{1, -1, 35, 36, 37, -1, 40}, New(400, 10, 36).Pages())
		assert.Equal(t, []int{1, -1, 36, 37, 38, 39, 40}, New(400, 10, 37).Pages())
	})
}
