package forms

import (
	"net/http"

	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/context"
	"gitea.com/go-chi/binding"
)

type StarListForm struct {
	Action string `binding:"Required"`
	ID     int64  `binding:"OmitEmpty"`
	UID    int64  `binding:"Required"`
	Name   string `binding:"Required;MaxSize(50)"`
	Desc   string `binding:"OmitEmpty"`
}

func (s *StarListForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, s, ctx.Locale)
}
