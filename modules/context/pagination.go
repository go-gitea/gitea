// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/paginator"
)

// Pagination provides a pagination via paginator.Paginator and additional configurations for the link params used in rendering
type Pagination struct {
	Paginater *paginator.Paginator
	urlParams []string
}

// NewPagination creates a new instance of the Pagination struct.
// "pagingNum" is "page size" or "limit", "current" is "page"
func NewPagination(total, pagingNum, current, numPages int) *Pagination {
	p := &Pagination{}
	p.Paginater = paginator.New(total, pagingNum, current, numPages)
	return p
}

// AddParam adds a value from context identified by ctxKey as link param under a given paramKey
func (p *Pagination) AddParam(ctx *Context, paramKey, ctxKey string) {
	_, exists := ctx.Data[ctxKey]
	if !exists {
		return
	}
	paramData := fmt.Sprintf("%v", ctx.Data[ctxKey]) // cast any to string
	urlParam := fmt.Sprintf("%s=%v", url.QueryEscape(paramKey), url.QueryEscape(paramData))
	p.urlParams = append(p.urlParams, urlParam)
}

// AddParamString adds a string parameter directly
func (p *Pagination) AddParamString(key, value string) {
	urlParam := fmt.Sprintf("%s=%v", url.QueryEscape(key), url.QueryEscape(value))
	p.urlParams = append(p.urlParams, urlParam)
}

// GetParams returns the configured URL params
func (p *Pagination) GetParams() template.URL {
	return template.URL(strings.Join(p.urlParams, "&"))
}

// SetDefaultParams sets common pagination params that are often used
func (p *Pagination) SetDefaultParams(ctx *Context) {
	p.AddParam(ctx, "sort", "SortType")
	p.AddParam(ctx, "q", "Keyword")
	// do not add any more uncommon params here!
	p.AddParam(ctx, "t", "queryType")
}
