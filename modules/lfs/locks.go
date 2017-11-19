package lfs

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/sdk/gitea"

	"gopkg.in/macaron.v1"
)

func checkRequest(req macaron.Request) int {
	if !setting.LFS.StartServer {
		return 404
	}
	if !ContentMatcher(req) || req.Header.Get("Content-Type") != contentMediaType {
		return 400
	}
	return 200
}

// GetLockHandler list locks
func GetLockHandler(ctx *context.Context) {
	status := checkRequest(ctx.Req)
	if status != 200 {
		writeStatus(ctx, status)
		return
	}
	ctx.Resp.Header().Set("Content-Type", metaMediaType)
	ctx.JSON(404, api.LockError{Message: "Not found"})
}

// PostLockHandler create lock
func PostLockHandler(ctx *context.Context) {
	status := checkRequest(ctx.Req)
	if status != 200 {
		writeStatus(ctx, status)
		return
	}
	ctx.Resp.Header().Set("Content-Type", metaMediaType)
	ctx.JSON(404, api.LockError{Message: "Not found"})
}

// VerifyLockHandler list locks for verification
func VerifyLockHandler(ctx *context.Context) {
	status := checkRequest(ctx.Req)
	if status != 200 {
		writeStatus(ctx, status)
		return
	}
	ctx.Resp.Header().Set("Content-Type", metaMediaType)
	ctx.JSON(404, api.LockError{Message: "Not found"})
}

// UnLockHandler delete locks
func UnLockHandler(ctx *context.Context) {
	status := checkRequest(ctx.Req)
	if status != 200 {
		writeStatus(ctx, status)
		return
	}
	ctx.Resp.Header().Set("Content-Type", metaMediaType)
	ctx.JSON(404, api.LockError{Message: "Not found"})
}
