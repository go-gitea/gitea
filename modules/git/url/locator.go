// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package url

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"gitea.dev/modules/util"
)

// Locator holds information needed to build various repository paths
type Locator struct {
	Owner   string
	Repo    string
	GroupID int64
}

func (l Locator) groupSegment() string {
	return util.Iif(l.GroupID > 0, fmt.Sprintf("group/%d", l.GroupID), "")
}

func (l Locator) groupSegmentWithTrailingSlash() string {
	return util.Iif(l.GroupID > 0, l.groupSegment()+"/", "")
}

func (l Locator) StoragePath() string {
	seg := util.Iif(l.GroupID > 0, strconv.FormatInt(l.GroupID, 10)+"/", "")
	return strings.ToLower(l.Owner) + "/" + seg + strings.ToLower(l.Repo)
}

func (l Locator) WebPath() string {
	return url.PathEscape(l.Owner) + "/" + l.groupSegmentWithTrailingSlash() + url.PathEscape(l.Repo)
}

func (l Locator) FullName() string {
	return l.Owner + "/" + l.groupSegmentWithTrailingSlash() + l.Repo
}

func NewLocator(ownerName, repoName string, groupID int64) Locator {
	return Locator{
		Owner:   ownerName,
		Repo:    repoName,
		GroupID: groupID,
	}
}
