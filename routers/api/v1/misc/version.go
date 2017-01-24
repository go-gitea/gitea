package misc

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

// Version of the Gitea server
func ServerVersion(ctx *context.APIContext) {
	ctx.Write([]byte(setting.AppVer))
}
