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
			// here, you  could also try to cast the ctx to a modules/context.Context
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
