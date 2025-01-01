// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
)

func checkUserType(ctx context.Context, logger log.Logger, autofix bool) error {
	count, err := user_model.CountWrongUserType(ctx)
	if err != nil {
		logger.Critical("Error: %v whilst counting wrong user types")
		return err
	}
	if count > 0 {
		if autofix {
			if count, err = user_model.FixWrongUserType(ctx); err != nil {
				logger.Critical("Error: %v whilst fixing wrong user types")
				return err
			}
			logger.Info("%d users with wrong type fixed", count)
		} else {
			logger.Warn("%d users with wrong type exist", count)
		}
	}
	return nil
}

func init() {
	Register(&Check{
		Title:     "Check if user with wrong type exist",
		Name:      "check-user-type",
		IsDefault: true,
		Run:       checkUserType,
		Priority:  3,
	})
}
