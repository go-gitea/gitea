// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"net/url"
	"reflect"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
)

// GetQueryBeforeSince return parsed time (unix format) from URL query's before and since
func GetQueryBeforeSince(ctx *context.APIContext) (before, since int64, err error) {
	qCreatedBefore, err := prepareQueryArg(ctx, "before")
	if err != nil {
		return 0, 0, err
	}

	qCreatedSince, err := prepareQueryArg(ctx, "since")
	if err != nil {
		return 0, 0, err
	}

	before, err = parseTime(qCreatedBefore)
	if err != nil {
		return 0, 0, err
	}

	since, err = parseTime(qCreatedSince)
	if err != nil {
		return 0, 0, err
	}
	return before, since, nil
}

// parseTime parse time and return unix timestamp
func parseTime(value string) (int64, error) {
	if len(value) != 0 {
		t, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return 0, err
		}
		if !t.IsZero() {
			return t.Unix(), nil
		}
	}
	return 0, nil
}

// prepareQueryArg unescape and trim a query arg
func prepareQueryArg(ctx *context.APIContext, name string) (value string, err error) {
	value, err = url.PathUnescape(ctx.Query(name))
	value = strings.Trim(value, " ")
	return
}

// GetListOptions returns list options using the page and limit parameters
func GetListOptions(ctx *context.APIContext) models.ListOptions {
	return models.ListOptions{
		Page:     ctx.QueryInt("page"),
		PageSize: convert.ToCorrectPageSize(ctx.QueryInt("limit")),
	}
}

// PaginateSlice cut a slice as per pagination options
// if page = 0 it do not paginate
func PaginateSlice(list interface{}, page, pageSize int) interface{} {
	if page <= 0 || pageSize <= 0 {
		return list
	}
	listValue := reflect.ValueOf(list)

	if reflect.TypeOf(list).Kind() != reflect.Slice {
		return list
	}

	page--

	if page*pageSize >= listValue.Len() {
		return listValue.Slice(listValue.Len(), listValue.Len()).Interface()
	}

	listValue = listValue.Slice(page*pageSize, listValue.Len())

	if listValue.Len() > pageSize {
		return listValue.Slice(0, pageSize).Interface()
	}

	return listValue.Interface()
}
