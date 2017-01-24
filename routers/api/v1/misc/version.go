package misc

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

// ServerVersion shows the version of the Gitea server
func ServerVersion(ctx *context.APIContext) {
	ctx.Write([]byte(setting.AppVer))
}
