package misc

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
)

// SigningKey returns the public key of the default signing key if it exists
func SigningKey(ctx *context.Context) {
	signingKey, _ := git.NewCommand("config", "--get", "user.signingkey").Run()
	signingKey = strings.TrimSpace(signingKey)
	if len(signingKey) == 0 {
		_, err := ctx.Write([]byte{})
		if err != nil {
			log.Error("Error Writing empty string %v", err)
			ctx.Error(http.StatusInternalServerError, fmt.Sprintf("%v", err))
		}
		return
	}

	content, stderr, err := process.GetManager().Exec(
		"gpg --export -a", "gpg", "--export", "-a", signingKey)
	if err != nil {
		log.Error("Unable to get default signing key: %s, %s, %v", signingKey, stderr, err)
		ctx.ServerError("gpg export", err)
		return
	}
	_, err = ctx.Write([]byte(content))
	if err != nil {
		log.Error("Error writing key content %v", err)
		ctx.Error(http.StatusInternalServerError, fmt.Sprintf("%v", err))
	}
}
