// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"
	"encoding/json"
	"fmt"

	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/services/pubsub"
)

func (n *websocketNotifier) DeleteComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment) {
	d, err := json.Marshal(c)
	if err != nil {
		return
	}

	n.pubsub.Publish(ctx, pubsub.Message{
		Data:  d,
		Topic: fmt.Sprintf("repo:%s/%s", c.RefRepo.OwnerName, c.RefRepo.Name),
	})
}
