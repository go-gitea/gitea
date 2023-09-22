// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "code.gitea.io/gitea/internal/modules/structs"
)

// ActivityPub
// swagger:response ActivityPub
type swaggerResponseActivityPub struct {
	// in:body
	Body api.ActivityPub `json:"body"`
}
