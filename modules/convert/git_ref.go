// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"net/url"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
)

// ToGitRef converts a git.Reference to a api.Reference
func ToGitRef(repo *repo_model.Repository, ref *git.Reference) *api.Reference {
	return &api.Reference{
		Ref: ref.Name,
		URL: repo.APIURL() + "/git/" + util.PathEscapeSegments(ref.Name),
		Object: &api.GitObject{
			SHA:  ref.Object.String(),
			Type: ref.Type,
			URL:  repo.APIURL() + "/git/" + url.PathEscape(ref.Type) + "s/" + url.PathEscape(ref.Object.String()),
		},
	}
}
