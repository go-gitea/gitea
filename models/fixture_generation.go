// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"
)

// GetYamlFixturesUserRepoUnit returns a string containing the contents for the
// user_repo_unit table, as recalculated using repo.RebuildAllUserRepoUnits()
func GetYamlFixturesUserRepoUnit() (string, error) {

	if err := RebuildAllUserRepoUnits(x); err != nil {
		return "", err
	}

	var b strings.Builder

	units := make([]*UserRepoUnit, 0, 200)
	if err := x.OrderBy("user_id, repo_id").Find(&units); err != nil {
		return "", err
	}

	for _, u := range units {
		fmt.Fprintf(&b, "-\n")
		fmt.Fprintf(&b, "  user_id: %d\n", u.UserID)
		fmt.Fprintf(&b, "  repo_id: %d\n", u.RepoID)
		fmt.Fprintf(&b, "  type: %d\n", u.Type)
		fmt.Fprintf(&b, "  mode: %d\n", u.Mode)
		fmt.Fprintf(&b, "\n")
	}

	return b.String(), nil
}
