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
		assert.Empty(t, pages) // no "total", so no pages

		p = New(1, 10, 1, 1)
		pages = p.Pages()
		assert.Len(t, pages, 1)
		assert.Equal(t, 1, pages[0].Num())
		assert.True(t, pages[0].IsCurrent())
	})

	assertPages := func(t *testing.T, total, current, numPages int, expected ...int) {
		pages := New(total, 10, current, numPages).Pages()
		assert.Len(t, pages, len(expected))
		for i, number := range expected {
			assert.Equal(t, number, pages[i].Num())
			if number == current {
				assert.True(t, pages[i].IsCurrent())
			} else {
				assert.False(t, pages[i].IsCurrent())
			}
		}
	}

	t.Run("Total page number is less or equal", func(t *testing.T) {
		assertPages(t, 1, 1, 2, 1)

		assertPages(t, 11, 1, 2, 1, 2)

		assertPages(t, 11, 2, 2, 1, 2)

		assertPages(t, 25, 2, 3, 1, 2, 3)
	})

	t.Run("Has more previous pages ", func(t *testing.T) {
		// ... 2
		assertPages(t, 11, 2, 1, -1, 2)

		// ... 2 3
		assertPages(t, 21, 2, 2, -1, 2, 3)

		// ... 2 3 4
		assertPages(t, 31, 3, 3, -1, 2, 3, 4)

		// ... 3 4 5
		assertPages(t, 41, 4, 3, -1, 3, 4, 5)

		// ... 4 5 6 7 8 9 10
		assertPages(t, 100, 9, 7, -1, 4, 5, 6, 7, 8, 9, 10)
	})

	t.Run("Has more next pages", func(t *testing.T) {
		// 1 ...
		assertPages(t, 21, 1, 1, 1, -1)

		// 1 2 ...
		assertPages(t, 21, 1, 2, 1, 2, -1)

		// 1 2 3 ...
		assertPages(t, 31, 2, 3, 1, 2, 3, -1)

		// 1 2 3 ...
		assertPages(t, 41, 2, 3, 1, 2, 3, -1)

		// 1 2 3 4 5 6 7 ...
		assertPages(t, 100, 1, 7, 1, 2, 3, 4, 5, 6, 7, -1)

		// 1 2 3 4 5 6 7 ...
		assertPages(t, 100, 2, 7, 1, 2, 3, 4, 5, 6, 7, -1)
	})

	t.Run("Has both more previous and next pages", func(t *testing.T) {
		// ... 2 3 ...
		assertPages(t, 35, 2, 2, -1, 2, 3, -1)

		// ... 2 3 4 ...
		assertPages(t, 49, 3, 3, -1, 2, 3, 4, -1)
	})
}
