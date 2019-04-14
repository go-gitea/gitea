// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"html/template"
	"net/url"

	"github.com/Unknwon/paginater"
)

// Pagination provides a pagination via Paginater and additional configurations for the link params used in rendering
type Pagination struct {
	Paginater            *paginater.Paginater
	paginationLinkParams template.URL
}

// NewPagination creates a new instance of the Pagination struct
func NewPagination(total int, page int, issueNum int, numPages int) *Pagination {
	p := &Pagination{}
	p.Paginater = paginater.New(total, page, issueNum, numPages)
	p.paginationLinkParams = ""
	return p
}

// AddParam adds a value from context identified by ctxKey as link param under a given paramKey
func (p *Pagination) AddParam(ctx *Context, paramKey string, ctxKey string) {
	_, exists := ctx.Data[ctxKey]
	if !exists {
		return
	}
	p.paginationLinkParams = template.URL(fmt.Sprintf("%v%s=%v&", p.paginationLinkParams, url.QueryEscape(paramKey), url.QueryEscape(ctx.Data[ctxKey])))
}

// GetParams returns the configured URL params
func (p *Pagination) GetParams() template.URL {
	return p.paginationLinkParams
}

// SetDefaultParams sets common pagination params that are often used
func (p *Pagination) SetDefaultParams(ctx *Context) {
	p.AddParam(ctx, "sort", "SortType")
	p.AddParam(ctx, "q", "Keyword")
	p.AddParam(ctx, "tab", "TabName")
}
