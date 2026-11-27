// Copyright 2022 The Gitea Authors.
// Copyright 2015 https://github.com/unknwon. Licensed under the Apache License, Version 2.0
// SPDX-License-Identifier: Apache-2.0

package paginator

import "gitea.dev/modules/util"

// Paginator represents a set of results of pagination calculations.
type Paginator struct {
	total      int // total rows count, -1 means unknown
	totalPages int // total pages count, -1 means unknown
	current    int // current page number

	pagingNum int // how many rows in one page

	hasNext *bool // used for total=-1 ("unlimited paging")
}

// New initialize a new pagination calculation and returns a Paginator as result.
func New(total, pagingNum, current int) *Paginator {
	pagingNum = max(pagingNum, 1)
	totalPages := util.Iif(total == -1, -1, (total+pagingNum-1)/pagingNum)
	if total >= 0 {
		current = min(current, totalPages)
	}
	current = max(current, 1)
	return &Paginator{
		total:      total,
		totalPages: totalPages,
		current:    current,
		pagingNum:  pagingNum,
	}
}

func (p *Paginator) SetUnlimitedPaging(curRows int, hasNext bool) {
	// For "unlimited paging", we need to know the rows of current page to determine if there is a next page.
	p.hasNext = &hasNext
	if p.total == -1 && p.current == 1 && !hasNext {
		// if there is only one page for the "unlimited paging", set total rows/pages count
		// then the tmpl could decide to hide the nav bar.
		p.total = curRows
		p.totalPages = util.Iif(p.total == 0, 0, 1)
	}
}

// IsFirst returns true if current page is the first page.
func (p *Paginator) IsFirst() bool {
	return p.current == 1
}

// HasPrevious returns true if there is a previous page relative to current page.
func (p *Paginator) HasPrevious() bool {
	return p.current > 1
}

func (p *Paginator) Previous() int {
	if !p.HasPrevious() {
		return p.current
	}
	return p.current - 1
}

// HasNext returns true if there is a next page relative to current page.
func (p *Paginator) HasNext() bool {
	if p.hasNext != nil {
		return *p.hasNext
	}
	return p.current*p.pagingNum < p.total
}

func (p *Paginator) Next() int {
	if !p.HasNext() {
		return p.current
	}
	return p.current + 1
}

// IsLast returns true if current page is the last page.
func (p *Paginator) IsLast() bool {
	return !p.HasNext()
}

// Total returns number of total rows.
func (p *Paginator) Total() int {
	return p.total
}

// TotalPages returns number of total pages.
func (p *Paginator) TotalPages() int {
	return p.totalPages
}

// Current returns current page number.
func (p *Paginator) Current() int {
	return p.current
}

// PagingNum returns number of page size.
func (p *Paginator) PagingNum() int {
	return p.pagingNum
}

// Pages returns the page numbers to show with -1 for a gap, mirrored by getPaginationPages in web_src/js/utils.ts
func (p *Paginator) Pages() []int {
	total, current := p.TotalPages(), p.current
	switch {
	case total == -1:
		return []int{current}
	case total <= 7:
		return []int{1, 2, 3, 4, 5, 6, 7}[:max(total, 0)]
	case current <= 4:
		return []int{1, 2, 3, 4, 5, -1, total}
	case current >= total-3:
		return []int{1, -1, total - 4, total - 3, total - 2, total - 1, total}
	default:
		return []int{1, -1, current - 1, current, current + 1, -1, total}
	}
}
