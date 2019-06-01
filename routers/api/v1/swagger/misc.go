// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// ServerVersion
// swagger:response ServerVersion
type swaggerResponseServerVersion struct {
	// in:body
	Body api.ServerVersion `json:"body"`
}
