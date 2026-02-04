// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

func SetMustChangePassword(ctx context.Context, all, mustChangePassword bool, include, exclude []string) (int64, error) {
	sliceTrimSpaceDropEmpty := func(input []string) []string {
		output := make([]string, 0, len(input))
		for _, in := range input {
			in = strings.ToLower(strings.TrimSpace(in))
			if in == "" {
				continue
			}
			output = append(output, in)
		}
		return output
	}

	var cond builder.Cond

	// Only include the users where something changes to get an accurate count
	cond = builder.Neq{"must_change_password": mustChangePassword}

	if !all {
		include = sliceTrimSpaceDropEmpty(include)
		if len(include) == 0 {
			return 0, util.ErrorWrap(util.ErrInvalidArgument, "no users to include provided")
		}

		cond = cond.And(builder.In("lower_name", include))
	}

	exclude = sliceTrimSpaceDropEmpty(exclude)
	if len(exclude) > 0 {
		cond = cond.And(builder.NotIn("lower_name", exclude))
	}

	return db.GetEngine(ctx).Where(cond).MustCols("must_change_password").Update(&User{MustChangePassword: mustChangePassword})
}
