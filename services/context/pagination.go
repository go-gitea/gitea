// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"gitea.dev/modules/container"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/paginator"
)

type PaginationBuilder struct {
	ctx          *Context
	total        int64
	curPage      int
	perPageLimit int
	navPageNum   *int
}

func NewPaginationBuilder(ctx *Context) *PaginationBuilder {
	return &PaginationBuilder{ctx: ctx}
}

func (pb *PaginationBuilder) TotalCount(n int64) *PaginationBuilder {
	pb.total = n
	return pb
}

func (pb *PaginationBuilder) PerPageLimit(n int) *PaginationBuilder {
	pb.perPageLimit = n
	return pb
}

func (pb *PaginationBuilder) CurPage(n int) *PaginationBuilder {
	pb.curPage = n
	return pb
}

func (pb *PaginationBuilder) NavPageNum(n int) *PaginationBuilder {
	pb.navPageNum = &n
	return pb
}

func (pb *PaginationBuilder) Build() *Pagination {
	navPageNum := optional.FromPtr(pb.navPageNum).ValueOrDefault(5)
	p := NewPagination(pb.total, pb.perPageLimit, pb.curPage, navPageNum)
	p.AddParamFromRequest(pb.ctx.Req)
	return p
}

// Pagination provides a pagination via paginator.Paginator and additional configurations for the link params used in rendering
type Pagination struct {
	Paginator *paginator.Paginator
	urlParams []string
}

// NewPagination creates a new instance of the Pagination struct.
// "total" is usually from database result "count int64", so it also uses int64
// "pagingNum" is "page size" or "limit", "current" is "page"
// total=-1 means only showing prev/next
func NewPagination(total int64, pagingNum, current, numPages int) *Pagination {
	totalInt := int(min(total, int64(math.MaxInt)))
	p := &Pagination{}
	p.Paginator = paginator.New(totalInt, pagingNum, current, numPages)
	return p
}

func (p *Pagination) WithUnlimitedPaging(curRows int, hasNext bool) *Pagination {
	p.Paginator.SetUnlimitedPaging(curRows, hasNext)
	return p
}

func (p *Pagination) AddParamFromQuery(q url.Values) {
	for key, values := range q {
		if key == "page" || len(values) == 0 || (len(values) == 1 && values[0] == "") {
			continue
		}
		for _, value := range values {
			urlParam := fmt.Sprintf("%s=%v", url.QueryEscape(key), url.QueryEscape(value))
			p.urlParams = append(p.urlParams, urlParam)
		}
	}
}

func (p *Pagination) AddParamFromRequest(req *http.Request) {
	p.AddParamFromQuery(req.URL.Query())
}

func (p *Pagination) RemoveParam(keys container.Set[string]) {
	p.urlParams = slices.DeleteFunc(p.urlParams, func(s string) bool {
		k, _, _ := strings.Cut(s, "=")
		k, _ = url.QueryUnescape(k)
		return keys.Contains(k)
	})
}

// GetParams returns the configured URL params
func (p *Pagination) GetParams() template.URL {
	return template.URL(strings.Join(p.urlParams, "&"))
}
