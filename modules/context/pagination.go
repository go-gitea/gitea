package context

import (
	"fmt"
	"html/template"
)

func AddPaginationParam(ctx *Context, paramKey string, ctxKey string) {
	_, exists := ctx.Data[ctxKey]
	if !exists {
		return
	}
	if ctx.Data["PaginationLinkAppend"] == nil {
		ClearPaginationParam(ctx)
	}
	ctx.Data["PaginationLinkAppend"] = template.URL(fmt.Sprintf("%v%s=%v&", ctx.Data["PaginationLinkAppend"], paramKey, ctx.Data[ctxKey]))
}

func ClearPaginationParam(ctx *Context) {
	ctx.Data["PaginationLinkAppend"] = ""
}

func DefaultPaginationParams(ctx *Context) {
	ClearPaginationParam(ctx)
	AddPaginationParam(ctx, "sort", "SortType")
	AddPaginationParam(ctx, "q", "Keyword")
	AddPaginationParam(ctx, "tab", "TabName")
}
