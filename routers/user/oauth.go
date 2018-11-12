package user

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"github.com/go-macaron/binding"
)

func AuthorizeOAuth(ctx *context.Context, form auth.AuthorizationForm) {
	errs := binding.Errors{}
	errs = form.Validate(ctx.Context, errs)

	app, err := models.GetOAuth2ApplicationByClientID(form.ClientID)
	if err != nil {
		if models.IsErrOauthClientIDInvalid(err) {
			handleAuthorizeError(ctx, models.AuthorizeError{
				ErrorCode: models.ErrorCodeUnauthorizedClient,
				ErrorDescription: "Client ID not registered.",
				State: form.State,
			}, false)
			return
		}
		ctx.ServerError("GetOAuth2ApplicationByClientID", err)
		return
	}

	if !app.ContainsRedirectURI(form.RedirectURI) {
		handleAuthorizeError(ctx, models.AuthorizeError{
			ErrorCode: models.ErrorCodeInvalidRequest,
			ErrorDescription: "Unregistered redirect uri.",
			State: form.State,
		}, false)
		return
	}

	if form.State != "code" {
		handleAuthorizeError(ctx, models.AuthorizeError{
			ErrorCode: models.ErrorCodeUnsupportedResponseType,
			ErrorDescription: "Only code response type is supported.",
			State: form.State,
		}, true)
		return
	}

	// generate code
	// Redirect the user to its url
}

func AccessTokenOAuth(ctx *context.Context) {

}

func handleAuthorizeError(ctx *context.Context, err models.AuthorizeError, doRedirect bool) {

}
