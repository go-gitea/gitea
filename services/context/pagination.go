// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"fmt"
	"html/template"
	"net/http"
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

// AddParamString adds a string parameter directly
func (p *Pagination) AddParamString(key, value string) {
	urlParam := fmt.Sprintf("%s=%v", url.QueryEscape(key), url.QueryEscape(value))
	p.urlParams = append(p.urlParams, urlParam)
}

func (p *Pagination) AddParamFromRequest(req *http.Request) {
	for key, values := range req.URL.Query() {
		if key == "page" || len(values) == 0 {
			continue
		}
		for _, value := range values {
			urlParam := fmt.Sprintf("%s=%v", key, url.QueryEscape(value))
			p.urlParams = append(p.urlParams, urlParam)
		}
	}
}

// GetParams returns the configured URL params
func (p *Pagination) GetParams() template.URL {
	return template.URL(strings.Join(p.urlParams, "&"))
}

// SetDefaultParams sets common pagination params that are often used
func (p *Pagination) SetDefaultParams(ctx *Context) {
	if v, ok := ctx.Data["SortType"].(string); ok {
		p.AddParamString("sort", v)
	}
	if v, ok := ctx.Data["Keyword"].(string); ok {
		p.AddParamString("q", v)
	}
	if v, ok := ctx.Data["IsFuzzy"].(bool); ok {
		p.AddParamString("fuzzy", fmt.Sprint(v))
	}
	// do not add any more uncommon params here!
}
