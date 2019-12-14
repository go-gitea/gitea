// Copyright 2015 Unknwon
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

// Package paginater is a helper module for custom pagination calculation.
package paginater

// Paginater represents a set of results of pagination calculations.
type Paginater struct {
	total     int
	pagingNum int
	current   int
	numPages  int
}

// New initialize a new pagination calculation and returns a Paginater as result.
func New(total, pagingNum, current, numPages int) *Paginater {
	if pagingNum <= 0 {
		pagingNum = 1
	}
	if current <= 0 {
		current = 1
	}
	p := &Paginater{total, pagingNum, current, numPages}
	if p.current > p.TotalPages() {
		p.current = p.TotalPages()
	}
	return p
}

// IsFirst returns true if current page is the first page.
func (p *Paginater) IsFirst() bool {
	return p.current == 1
}

// HasPrevious returns true if there is a previous page relative to current page.
func (p *Paginater) HasPrevious() bool {
	return p.current > 1
}

func (p *Paginater) Previous() int {
	if !p.HasPrevious() {
		return p.current
	}
	return p.current - 1
}

// HasNext returns true if there is a next page relative to current page.
func (p *Paginater) HasNext() bool {
	return p.total > p.current*p.pagingNum
}

func (p *Paginater) Next() int {
	if !p.HasNext() {
		return p.current
	}
	return p.current + 1
}

// IsLast returns true if current page is the last page.
func (p *Paginater) IsLast() bool {
	if p.total == 0 {
		return true
	}
	return p.total > (p.current-1)*p.pagingNum && !p.HasNext()
}

// Total returns number of total rows.
func (p *Paginater) Total() int {
	return p.total
}

// TotalPage returns number of total pages.
func (p *Paginater) TotalPages() int {
	if p.total == 0 {
		return 1
	}
	if p.total%p.pagingNum == 0 {
		return p.total / p.pagingNum
	}
	return p.total/p.pagingNum + 1
}

// Current returns current page number.
func (p *Paginater) Current() int {
	return p.current
}

// Page presents a page in the paginater.
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
	if numPages%2 == 0 {
		return numPages / 2
	}
	return numPages/2 + 1
}

// Pages returns a list of nearby page numbers relative to current page.
// If value is -1 means "..." that more pages are not showing.
func (p *Paginater) Pages() []*Page {
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
	maxIdx := numPages - 1
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
		maxIdx++
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
