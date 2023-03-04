// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"code.gitea.io/gitea/models/auth"
	auth_service "code.gitea.io/gitea/services/auth"
)

// specialAdd registers the SSPI auth method as the last method in the list.
// The SSPI plugin is expected to be executed last, as it returns 401 status code if negotiation
// fails (or if negotiation should continue), which would prevent other authentication methods
// to execute at all.
func specialAdd(group *auth_service.Group) {
	if auth.IsSSPIEnabled() {
		group.Add(&auth_service.SSPI{})
	}
}
