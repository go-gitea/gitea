package user

import (
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"github.com/go-macaron/binding"
)

func AuthorizeOAuth(ctx *context.Context, form auth.AuthorizationForm) {
	errs := binding.Errors{}
	errs = form.Validate(ctx.Context, errs)
}

func AccessTokenOAuth(ctx *context.Context) {

}
