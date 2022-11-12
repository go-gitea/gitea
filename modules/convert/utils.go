// Copyright 2020 The Gitea Authors. All rights reserved.
// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
)

// ToCorrectPageSize makes sure page size is in allowed range.
func ToCorrectPageSize(size int) int {
	if size <= 0 {
		size = setting.API.DefaultPagingNum
	} else if size > setting.API.MaxResponseItems {
		size = setting.API.MaxResponseItems
	}
	return size
}

// ToGitServiceType return GitServiceType based on string
func ToGitServiceType(value string) structs.GitServiceType {
	switch strings.ToLower(value) {
	case "github":
		return structs.GithubService
	case "gitea":
		return structs.GiteaService
	case "gitlab":
		return structs.GitlabService
	case "gogs":
		return structs.GogsService
	case "onedev":
		return structs.OneDevService
	case "gitbucket":
		return structs.GitBucketService
	default:
		return structs.PlainGitService
	}
}
