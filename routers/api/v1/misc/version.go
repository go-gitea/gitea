// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package misc

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

// ServerVersion shows the version of the Gitea server
func ServerVersion(ctx *context.APIContext) {
	ctx.Write([]byte(setting.AppVer))
}
