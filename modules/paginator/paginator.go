// Copyright 2022 The Gitea Authors.
// Copyright 2015 https://github.com/unknwon. Licensed under the Apache License, Version 2.0

package paginator

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
	total     int // total rows count
	pagingNum int // how many rows in one page
	current   int // current page number
	numPages  int // how many pages to show on the UI
}

// New initialize a new pagination calculation and returns a Paginator as result.
func New(total, pagingNum, current, numPages int) *Paginator {
	if pagingNum <= 0 {
		pagingNum = 1
	}
	if current <= 0 {
		current = 1
	}
	p := &Paginator{total, pagingNum, current, numPages}
	if p.current > p.TotalPages() {
		p.current = p.TotalPages()
	}
	return p
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
	return p.total > p.current*p.pagingNum
}

func (p *Paginator) Next() int {
	if !p.HasNext() {
		return p.current
	}
	return p.current + 1
}

// IsLast returns true if current page is the last page.
func (p *Paginator) IsLast() bool {
	if p.total == 0 {
		return true
	}
	return p.total > (p.current-1)*p.pagingNum && !p.HasNext()
}

// Total returns number of total rows.
func (p *Paginator) Total() int {
	return p.total
}

// TotalPages returns number of total pages.
func (p *Paginator) TotalPages() int {
	if p.total == 0 {
		return 1
	}
	return (p.total + p.pagingNum - 1) / p.pagingNum
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
		return []*Page{}
	} else if p.numPages == 1 && p.TotalPages() == 1 {
		// Only show current page.
		return []*Page{{1, true}}
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
