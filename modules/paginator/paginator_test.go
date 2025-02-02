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
		p := New(0, -1, -1, 0)
		assert.Equal(t, 1, p.PagingNum())
		assert.True(t, p.IsFirst())
		assert.False(t, p.HasPrevious())
		assert.Equal(t, 1, p.Previous())
		assert.False(t, p.HasNext())
		assert.Equal(t, 1, p.Next())
		assert.True(t, p.IsLast())
		assert.Equal(t, 0, p.Total())

		p = New(1, 10, 2, 0)
		assert.Equal(t, 10, p.PagingNum())
		assert.True(t, p.IsFirst())
		assert.False(t, p.HasPrevious())
		assert.False(t, p.HasNext())
		assert.True(t, p.IsLast())

		p = New(10, 10, 1, 0)
		assert.Equal(t, 10, p.PagingNum())
		assert.True(t, p.IsFirst())
		assert.False(t, p.HasPrevious())
		assert.False(t, p.HasNext())
		assert.True(t, p.IsLast())

		p = New(11, 10, 1, 0)
		assert.Equal(t, 10, p.PagingNum())
		assert.True(t, p.IsFirst())
		assert.False(t, p.HasPrevious())
		assert.True(t, p.HasNext())
		assert.Equal(t, 2, p.Next())
		assert.False(t, p.IsLast())

		p = New(11, 10, 2, 0)
		assert.Equal(t, 10, p.PagingNum())
		assert.False(t, p.IsFirst())
		assert.True(t, p.HasPrevious())
		assert.Equal(t, 1, p.Previous())
		assert.False(t, p.HasNext())
		assert.True(t, p.IsLast())

		p = New(20, 10, 2, 0)
		assert.Equal(t, 10, p.PagingNum())
		assert.False(t, p.IsFirst())
		assert.True(t, p.HasPrevious())
		assert.False(t, p.HasNext())
		assert.True(t, p.IsLast())

		p = New(25, 10, 2, 0)
		assert.Equal(t, 10, p.PagingNum())
		assert.False(t, p.IsFirst())
		assert.True(t, p.HasPrevious())
		assert.True(t, p.HasNext())
		assert.False(t, p.IsLast())
	})

	t.Run("Generate pages", func(t *testing.T) {
		p := New(0, 10, 1, 0)
		pages := p.Pages()
		assert.Empty(t, pages)
	})

	t.Run("Only current page", func(t *testing.T) {
		p := New(0, 10, 1, 1)
		pages := p.Pages()
		assert.Len(t, pages, 1)
		assert.Equal(t, 1, pages[0].Num())
		assert.True(t, pages[0].IsCurrent())

		p = New(1, 10, 1, 1)
		pages = p.Pages()
		assert.Len(t, pages, 1)
		assert.Equal(t, 1, pages[0].Num())
		assert.True(t, pages[0].IsCurrent())
	})

	t.Run("Total page number is less or equal", func(t *testing.T) {
		p := New(1, 10, 1, 2)
		pages := p.Pages()
		assert.Len(t, pages, 1)
		assert.Equal(t, 1, pages[0].Num())
		assert.True(t, pages[0].IsCurrent())

		p = New(11, 10, 1, 2)
		pages = p.Pages()
		assert.Len(t, pages, 2)
		assert.Equal(t, 1, pages[0].Num())
		assert.True(t, pages[0].IsCurrent())
		assert.Equal(t, 2, pages[1].Num())
		assert.False(t, pages[1].IsCurrent())

		p = New(11, 10, 2, 2)
		pages = p.Pages()
		assert.Len(t, pages, 2)
		assert.Equal(t, 1, pages[0].Num())
		assert.False(t, pages[0].IsCurrent())
		assert.Equal(t, 2, pages[1].Num())
		assert.True(t, pages[1].IsCurrent())

		p = New(25, 10, 2, 3)
		pages = p.Pages()
		assert.Len(t, pages, 3)
		assert.Equal(t, 1, pages[0].Num())
		assert.False(t, pages[0].IsCurrent())
		assert.Equal(t, 2, pages[1].Num())
		assert.True(t, pages[1].IsCurrent())
		assert.Equal(t, 3, pages[2].Num())
		assert.False(t, pages[2].IsCurrent())
	})

	t.Run("Has more previous pages ", func(t *testing.T) {
		// ... 2
		p := New(11, 10, 2, 1)
		pages := p.Pages()
		assert.Len(t, pages, 2)
		assert.Equal(t, -1, pages[0].Num())
		assert.False(t, pages[0].IsCurrent())
		assert.Equal(t, 2, pages[1].Num())
		assert.True(t, pages[1].IsCurrent())

		// ... 2 3
		p = New(21, 10, 2, 2)
		pages = p.Pages()
		assert.Len(t, pages, 3)
		assert.Equal(t, -1, pages[0].Num())
		assert.False(t, pages[0].IsCurrent())
		assert.Equal(t, 2, pages[1].Num())
		assert.True(t, pages[1].IsCurrent())
		assert.Equal(t, 3, pages[2].Num())
		assert.False(t, pages[2].IsCurrent())

		// ... 2 3 4
		p = New(31, 10, 3, 3)
		pages = p.Pages()
		assert.Len(t, pages, 4)
		assert.Equal(t, -1, pages[0].Num())
		assert.False(t, pages[0].IsCurrent())
		assert.Equal(t, 2, pages[1].Num())
		assert.False(t, pages[1].IsCurrent())
		assert.Equal(t, 3, pages[2].Num())
		assert.True(t, pages[2].IsCurrent())
		assert.Equal(t, 4, pages[3].Num())
		assert.False(t, pages[3].IsCurrent())

		// ... 3 4 5
		p = New(41, 10, 4, 3)
		pages = p.Pages()
		assert.Len(t, pages, 4)
		assert.Equal(t, -1, pages[0].Num())
		assert.False(t, pages[0].IsCurrent())
		assert.Equal(t, 3, pages[1].Num())
		assert.False(t, pages[1].IsCurrent())
		assert.Equal(t, 4, pages[2].Num())
		assert.True(t, pages[2].IsCurrent())
		assert.Equal(t, 5, pages[3].Num())
		assert.False(t, pages[3].IsCurrent())

		// ... 4 5 6 7 8 9 10
		p = New(100, 10, 9, 7)
		pages = p.Pages()
		assert.Len(t, pages, 8)
		assert.Equal(t, -1, pages[0].Num())
		assert.False(t, pages[0].IsCurrent())
		assert.Equal(t, 4, pages[1].Num())
		assert.False(t, pages[1].IsCurrent())
		assert.Equal(t, 5, pages[2].Num())
		assert.False(t, pages[2].IsCurrent())
		assert.Equal(t, 6, pages[3].Num())
		assert.False(t, pages[3].IsCurrent())
		assert.Equal(t, 7, pages[4].Num())
		assert.False(t, pages[4].IsCurrent())
		assert.Equal(t, 8, pages[5].Num())
		assert.False(t, pages[5].IsCurrent())
		assert.Equal(t, 9, pages[6].Num())
		assert.True(t, pages[6].IsCurrent())
		assert.Equal(t, 10, pages[7].Num())
		assert.False(t, pages[7].IsCurrent())
	})

	t.Run("Has more next pages", func(t *testing.T) {
		// 1 ...
		p := New(21, 10, 1, 1)
		pages := p.Pages()
		assert.Len(t, pages, 2)
		assert.Equal(t, 1, pages[0].Num())
		assert.True(t, pages[0].IsCurrent())
		assert.Equal(t, -1, pages[1].Num())
		assert.False(t, pages[1].IsCurrent())

		// 1 2 ...
		p = New(21, 10, 1, 2)
		pages = p.Pages()
		assert.Len(t, pages, 3)
		assert.Equal(t, 1, pages[0].Num())
		assert.True(t, pages[0].IsCurrent())
		assert.Equal(t, 2, pages[1].Num())
		assert.False(t, pages[1].IsCurrent())
		assert.Equal(t, -1, pages[2].Num())
		assert.False(t, pages[2].IsCurrent())

		// 1 2 3 ...
		p = New(31, 10, 2, 3)
		pages = p.Pages()
		assert.Len(t, pages, 4)
		assert.Equal(t, 1, pages[0].Num())
		assert.False(t, pages[0].IsCurrent())
		assert.Equal(t, 2, pages[1].Num())
		assert.True(t, pages[1].IsCurrent())
		assert.Equal(t, 3, pages[2].Num())
		assert.False(t, pages[2].IsCurrent())
		assert.Equal(t, -1, pages[3].Num())
		assert.False(t, pages[3].IsCurrent())

		// 1 2 3 ...
		p = New(41, 10, 2, 3)
		pages = p.Pages()
		assert.Len(t, pages, 4)
		assert.Equal(t, 1, pages[0].Num())
		assert.False(t, pages[0].IsCurrent())
		assert.Equal(t, 2, pages[1].Num())
		assert.True(t, pages[1].IsCurrent())
		assert.Equal(t, 3, pages[2].Num())
		assert.False(t, pages[2].IsCurrent())
		assert.Equal(t, -1, pages[3].Num())
		assert.False(t, pages[3].IsCurrent())

		// 1 2 3 4 5 6 7 ...
		p = New(100, 10, 1, 7)
		pages = p.Pages()
		assert.Len(t, pages, 8)
		assert.Equal(t, 1, pages[0].Num())
		assert.True(t, pages[0].IsCurrent())
		assert.Equal(t, 2, pages[1].Num())
		assert.False(t, pages[1].IsCurrent())
		assert.Equal(t, 3, pages[2].Num())
		assert.False(t, pages[2].IsCurrent())
		assert.Equal(t, 4, pages[3].Num())
		assert.False(t, pages[3].IsCurrent())
		assert.Equal(t, 5, pages[4].Num())
		assert.False(t, pages[4].IsCurrent())
		assert.Equal(t, 6, pages[5].Num())
		assert.False(t, pages[5].IsCurrent())
		assert.Equal(t, 7, pages[6].Num())
		assert.False(t, pages[6].IsCurrent())
		assert.Equal(t, -1, pages[7].Num())
		assert.False(t, pages[7].IsCurrent())

		// 1 2 3 4 5 6 7 ...
		p = New(100, 10, 2, 7)
		pages = p.Pages()
		assert.Len(t, pages, 8)
		assert.Equal(t, 1, pages[0].Num())
		assert.False(t, pages[0].IsCurrent())
		assert.Equal(t, 2, pages[1].Num())
		assert.True(t, pages[1].IsCurrent())
		assert.Equal(t, 3, pages[2].Num())
		assert.False(t, pages[2].IsCurrent())
		assert.Equal(t, 4, pages[3].Num())
		assert.False(t, pages[3].IsCurrent())
		assert.Equal(t, 5, pages[4].Num())
		assert.False(t, pages[4].IsCurrent())
		assert.Equal(t, 6, pages[5].Num())
		assert.False(t, pages[5].IsCurrent())
		assert.Equal(t, 7, pages[6].Num())
		assert.False(t, pages[6].IsCurrent())
		assert.Equal(t, -1, pages[7].Num())
		assert.False(t, pages[7].IsCurrent())
	})

	t.Run("Has both more previous and next pages", func(t *testing.T) {
		// ... 2 3 ...
		p := New(35, 10, 2, 2)
		pages := p.Pages()
		assert.Len(t, pages, 4)
		assert.Equal(t, -1, pages[0].Num())
		assert.False(t, pages[0].IsCurrent())
		assert.Equal(t, 2, pages[1].Num())
		assert.True(t, pages[1].IsCurrent())
		assert.Equal(t, 3, pages[2].Num())
		assert.False(t, pages[2].IsCurrent())
		assert.Equal(t, -1, pages[3].Num())
		assert.False(t, pages[3].IsCurrent())

		// ... 2 3 4 ...
		p = New(49, 10, 3, 3)
		pages = p.Pages()
		assert.Len(t, pages, 5)
		assert.Equal(t, -1, pages[0].Num())
		assert.False(t, pages[0].IsCurrent())
		assert.Equal(t, 2, pages[1].Num())
		assert.False(t, pages[1].IsCurrent())
		assert.Equal(t, 3, pages[2].Num())
		assert.True(t, pages[2].IsCurrent())
		assert.Equal(t, 4, pages[3].Num())
		assert.False(t, pages[3].IsCurrent())
		assert.Equal(t, -1, pages[4].Num())
		assert.False(t, pages[4].IsCurrent())
	})
}
