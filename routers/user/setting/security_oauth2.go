package setting

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// DeleteOAuth2Application deletes the given oauth2 application
func DeleteOAuth2Application(ctx *context.Context) {
	if err := models.DeleteOAuth2Application(ctx.QueryInt64("id"), ctx.User.ID); err != nil {
		ctx.ServerError("DeleteOAuth2Application", err)
		return
	}
	log.Trace("OAuth2 Application deleted: %s", ctx.User.Name)

	ctx.Flash.Success(ctx.Tr("settings.remove_oauth2_application_success"))
	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/applications",
	})
}
