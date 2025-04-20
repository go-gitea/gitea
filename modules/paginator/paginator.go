// Copyright 2022 The Gitea Authors.
// Copyright 2015 https://github.com/unknwon. Licensed under the Apache License, Version 2.0
// SPDX-License-Identifier: Apache-2.0

package paginator

import "code.gitea.io/gitea/modules/util"

/*
In template:

```html
{{if not .Page.IsFirst}}[First](1){{end}}
{{if .Page.HasPrevious}}[Previous]({{.Page.Previous}}){{end}}

{{range .Page.Pages}}
	{{if eq .Num -1}}
	...
	{{else}}
	{{.Num}}{{if .IsCurrent}}(current){{end}}
	{{end}}
{{end}}

{{if .Page.HasNext}}[Next]({{.Page.Next}}){{end}}
{{if not .Page.IsLast}}[Last]({{.Page.TotalPages}}){{end}}
```

Output:

```
[First](1) [Previous](2) ... 2 3(current) 4 ... [Next](4) [Last](5)
```
*/

// Paginator represents a set of results of pagination calculations.
type Paginator struct {
	total      int // total rows count, -1 means unknown
	totalPages int // total pages count, -1 means unknown
	current    int // current page number
	curRows    int // current page rows count

	pagingNum int // how many rows in one page
	numPages  int // how many pages to show on the UI
}

// New initialize a new pagination calculation and returns a Paginator as result.
func New(total, pagingNum, current, numPages int) *Paginator {
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
		numPages:   numPages,
	}
}

func (p *Paginator) SetCurRows(rows int) {
	// For "unlimited paging", we need to know the rows of current page to determine if there is a next page.
	// There is still an edge case: when curRows==pagingNum, then the "next page" will be an empty page.
	// Ideally we should query one more row to determine if there is really a next page, but it's impossible in current framework.
	p.curRows = rows
	if p.total == -1 && p.current == 1 && !p.HasNext() {
		// if there is only one page for the "unlimited paging", set total rows/pages count
		// then the tmpl could decide to hide the nav bar.
		p.total = rows
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
	if p.total == -1 {
		return p.curRows >= p.pagingNum
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

// Page presents a page in the paginator.
type Page struct {
	num       int
	isCurrent bool
}

func (p *Page) Num() int {
	return p.num
}

func (p *Page) IsCurrent() bool {
	return p.isCurrent
}

func getMiddleIdx(numPages int) int {
	return (numPages + 1) / 2
}

// Pages returns a list of nearby page numbers relative to current page.
// If value is -1 means "..." that more pages are not showing.
func (p *Paginator) Pages() []*Page {
	if p.numPages == 0 {
		return nil
	} else if p.total == -1 || (p.numPages == 1 && p.TotalPages() == 1) {
		// Only show current page.
		return []*Page{{p.current, true}}
	}

	// Total page number is less or equal.
	if p.TotalPages() <= p.numPages {
		pages := make([]*Page, p.TotalPages())
		for i := range pages {
			pages[i] = &Page{i + 1, i+1 == p.current}
		}
		return pages
	}

	numPages := p.numPages
	offsetIdx := 0
	hasMoreNext := false

	// Check more previous and next pages.
	previousNum := getMiddleIdx(p.numPages) - 1
	if previousNum > p.current-1 {
		previousNum -= previousNum - (p.current - 1)
	}
	nextNum := p.numPages - previousNum - 1
	if p.current+nextNum > p.TotalPages() {
		delta := nextNum - (p.TotalPages() - p.current)
		nextNum -= delta
		previousNum += delta
	}

	offsetVal := p.current - previousNum
	if offsetVal > 1 {
		numPages++
		offsetIdx = 1
	}

	if p.current+nextNum < p.TotalPages() {
		numPages++
		hasMoreNext = true
	}

	pages := make([]*Page, numPages)

	// There are more previous pages.
	if offsetIdx == 1 {
		pages[0] = &Page{-1, false}
	}
	// There are more next pages.
	if hasMoreNext {
		pages[len(pages)-1] = &Page{-1, false}
	}

	// Check previous pages.
	for i := 0; i < previousNum; i++ {
		pages[offsetIdx+i] = &Page{i + offsetVal, false}
	}

	pages[offsetIdx+previousNum] = &Page{p.current, true}

	// Check next pages.
	for i := 1; i <= nextNum; i++ {
		pages[offsetIdx+previousNum+i] = &Page{p.current + i, false}
	}

	return pages
}
