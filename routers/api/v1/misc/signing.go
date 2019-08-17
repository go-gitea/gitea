package misc

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

// SigningKey returns the public key of the default signing key if it exists
func SigningKey(ctx *context.Context) {
	path := ""
	if ctx.Repo != nil && ctx.Repo.Repository != nil {
		path = ctx.Repo.Repository.RepoPath()
	}

	content, err := models.PublicSigningKey(path)
	if err != nil {
		ctx.ServerError("gpg export", err)
		return
	}
	_, err = ctx.Write([]byte(content))
	if err != nil {
		log.Error("Error writing key content %v", err)
		ctx.Error(http.StatusInternalServerError, fmt.Sprintf("%v", err))
	}
}
