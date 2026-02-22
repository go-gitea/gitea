// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/paginator"
)

// Pagination provides a pagination via paginator.Paginator and additional configurations for the link params used in rendering
type Pagination struct {
	Paginater *paginator.Paginator
	urlParams []string
}

// NewPagination creates a new instance of the Pagination struct.
// "pagingNum" is "page size" or "limit", "current" is "page"
// total=-1 means only showing prev/next
func NewPagination(total, pagingNum, current, numPages int) *Pagination {
	p := &Pagination{}
	p.Paginater = paginator.New(total, pagingNum, current, numPages)
	return p
}

func (p *Pagination) WithCurRows(n int) *Pagination {
	p.Paginater.SetCurRows(n)
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
