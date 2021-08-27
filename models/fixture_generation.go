// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"
)

// GetYamlFixturesAccess returns a string containing the contents
// for the access table, as recalculated using repo.RecalculateAccesses()
func GetYamlFixturesAccess() (string, error) {
	repos := make([]*Repository, 0, 50)
	if err := x.Find(&repos); err != nil {
		return "", err
	}

	for _, repo := range repos {
		repo.MustOwner()
		if err := repo.RecalculateAccesses(); err != nil {
			return "", err
		}
	}

	var b strings.Builder

	accesses := make([]*Access, 0, 200)
	if err := x.OrderBy("user_id, repo_id").Find(&accesses); err != nil {
		return "", err
	}

	for i, a := range accesses {
		fmt.Fprintf(&b, "-\n")
		fmt.Fprintf(&b, "  id: %d\n", i+1)
		fmt.Fprintf(&b, "  user_id: %d\n", a.UserID)
		fmt.Fprintf(&b, "  repo_id: %d\n", a.RepoID)
		fmt.Fprintf(&b, "  mode: %d\n", a.Mode)
		fmt.Fprintf(&b, "\n")
	}

	return b.String(), nil
}
