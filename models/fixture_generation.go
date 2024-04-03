// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
)

// GetYamlFixturesAccess returns a string containing the contents
// for the access table, as recalculated using repo.RecalculateAccesses()
func GetYamlFixturesAccess(ctx context.Context) (string, error) {
	repos := make([]*repo_model.Repository, 0, 50)
	if err := db.GetEngine(ctx).Find(&repos); err != nil {
		return "", err
	}

	for _, repo := range repos {
		repo.MustOwner(ctx)
		if err := access_model.RecalculateAccesses(ctx, repo); err != nil {
			return "", err
		}
	}

	var b strings.Builder

	accesses := make([]*access_model.Access, 0, 200)
	if err := db.GetEngine(ctx).OrderBy("user_id, repo_id").Find(&accesses); err != nil {
		return "", err
	}

	for i, a := range accesses {
		fmt.Fprintf(&b, "-\n")
		fmt.Fprintf(&b, "  id: %d\n", i+1)
		fmt.Fprintf(&b, "  user_id: %d\n", a.UserID)
		fmt.Fprintf(&b, "  repo_id: %d\n", a.RepoID)
		fmt.Fprintf(&b, "  mode: %d\n", a.Mode)
		if i < len(accesses)-1 {
			fmt.Fprintf(&b, "\n")
		}
	}

	return b.String(), nil
}
