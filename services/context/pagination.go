// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/optional"
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
	obj, exists := ctx.Data[ctxKey]
	if !exists {
		return
	}
	// we check if the value in the context is an optional.Option type and either skip if it contains None
	// or unwrap it if it is Some
	if optVal, is := optional.ExtractValue(obj); is {
		if optVal == nil {
			// optional value is currently None
			return
		}
		obj = optVal
	}
	paramData := fmt.Sprintf("%v", obj) // cast any to string
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
