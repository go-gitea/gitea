// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"context"

	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
)

func ProcessorHelper() *markup.ProcessorHelper {
	return &markup.ProcessorHelper{
		IsUsernameMentionable: func(ctx context.Context, username string) bool {
			// TODO: cast ctx to modules/context.Context and use IsUserVisibleToViewer

			// Only link if the user actually exists
			userExists, err := user.IsUserExist(ctx, 0, username)
			if err != nil {
				log.Error("Failed to validate user in mention %q exists, assuming it does", username)
				userExists = true
			}
			return userExists
		},
	}
}
