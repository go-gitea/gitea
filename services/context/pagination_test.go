// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/url"
	"testing"

	"gitea.dev/modules/container"

	"github.com/stretchr/testify/assert"
)

func TestPagination(t *testing.T) {
	p := NewPagination(1, 1, 1, 1)
	params := url.Values{}
	params.Add("k1", "11")
	params.Add("k1", "12")
	params.Add("k", "a")
	params.Add("k", "b")
	params.Add("k2", "21")
	params.Add("k2", "22")
	params.Add("foo", "bar")

	p.AddParamFromQuery(params)
	v, _ := url.ParseQuery(string(p.GetParams()))
	assert.Equal(t, params, v)

	p.RemoveParam(container.SetOf("k", "foo"))
	params.Del("k")
	params.Del("foo")
	v, _ = url.ParseQuery(string(p.GetParams()))
	assert.Equal(t, params, v)

	p = NewPagination(-1, 1, 1, 1)
	p.WithUnlimitedPaging(0, false)
	assert.Zero(t, p.Paginater.TotalPages())
	assert.False(t, p.Paginater.HasNext())

	p = NewPagination(-1, 1, 1, 1)
	p.WithUnlimitedPaging(10, false)
	assert.Equal(t, 1, p.Paginater.TotalPages()) // first page, no next, so it should know that the total page number is 1
	assert.False(t, p.Paginater.HasNext())

	p = NewPagination(-1, 1, 2, 1)
	p.WithUnlimitedPaging(10, false)
	assert.Equal(t, -1, p.Paginater.TotalPages())
	assert.False(t, p.Paginater.HasNext())

	p = NewPagination(-1, 1, 1, 1)
	p.WithUnlimitedPaging(10, true)
	assert.Equal(t, -1, p.Paginater.TotalPages())
	assert.True(t, p.Paginater.HasNext())
}
