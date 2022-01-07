// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/url"
)

const defaultPageSize = 10
const maxPageSize = 50

// ListOptions options for using Gitea's API pagination
type ListOptions struct {
	Page     int
	PageSize int
}

func (o ListOptions) getURLQuery() url.Values {
	query := make(url.Values)
	query.Add("page", fmt.Sprintf("%d", o.Page))
	query.Add("limit", fmt.Sprintf("%d", o.PageSize))

	return query
}

// setDefaults set default pagination options if none or wrong are set
// if you set -1 as page it will set all to 0
func (o *ListOptions) setDefaults() {
	if o.Page < 0 {
		o.Page, o.PageSize = 0, 0
		return
	} else if o.Page == 0 {
		o.Page = 1
	}

	if o.PageSize < 0 || o.PageSize > maxPageSize {
		o.PageSize = defaultPageSize
	}
}
