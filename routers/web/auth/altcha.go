package auth

import (
	"code.gitea.io/gitea/modules/altcha"
	"code.gitea.io/gitea/services/context"
)

// GenerateAltchaChallenge generates a new ALTCHA challenge and returns it as JSON
func GenerateAltchaChallenge(ctx *context.Context) {
	challenge, err := altcha.GenerateChallenge(ctx)
	if err != nil {
		ctx.ServerError("GenerateAltchaChallenge", err)
		return
	}
	ctx.JSON(200, challenge)
}
