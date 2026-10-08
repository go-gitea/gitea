// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"gitea.dev/models/db" //nolint:depguard // allow to access db in migration
	"gitea.dev/modules/git/gitrepo"
)

// HINT: MIGRATION-STRUCT-FROZEN: the structs used in migrations should not be affected by the changes happen in the future,
// because the details can be different in different releases.
// e.g. if one migration uses "User" model, it works in the early releases,
// then one day, when the User model changes, the existing migration will break because it will use the new (incorrect) User model,
// it should only use the old User model.
//
// Related: "models", "modules/structs", git repo directory layout on the disk, etc.
//
// If changes happen, the old migrations need to use a snapshot struct/function (copy the code and freeze)

type ResourceIndex = db.ResourceIndex

func LocalCodeGitRepo(ownerName, repoName string) gitrepo.RepositoryFacade {
	return gitrepo.CodeRepoByName(ownerName, repoName)
}
