package lfs

import (
	"time"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"

	"gopkg.in/macaron.v1"
)

// Lock represent a lock
// for use with the locks API.
type Lock struct {
	ID       string     `json:"id"`
	Path     string     `json:"path"`
	LockedAt time.Time  `json:"locked_at"`
	Owner    *LockOwner `json:"owner"`
}

// LockOwner represent a lock owner
// for use with the locks API.
type LockOwner struct {
	Name string `json:"name"`
}

// LockRequest contains the path of the lock to create
// https://github.com/git-lfs/git-lfs/blob/master/docs/api/locking.md#create-lock
type LockRequest struct {
	Path string `json:"path"`
}

//LockResponse represent a lock created
// https://github.com/git-lfs/git-lfs/blob/master/docs/api/locking.md#create-lock
type LockResponse struct {
	Lock *Lock `json:"lock"`
}

//LockList represent a list of lock requested
// https://github.com/git-lfs/git-lfs/blob/master/docs/api/locking.md#list-locks
type LockList struct {
	Locks []*Lock `json:"locks"`
	Next  string  `json:"next_cursor,omitempty"`
}

//LockListVerify represent a list of lock verification requested
// https://github.com/git-lfs/git-lfs/blob/master/docs/api/locking.md#list-locks-for-verification
type LockListVerify struct {
	Ours   []*Lock `json:"ours"`
	Theirs []*Lock `json:"theirs"`
	Next   string  `json:"next_cursor,omitempty"`
}

// LockError contains information on the error that occurs
type LockError struct {
	Message       string `json:"message"`
	Lock          *Lock  `json:"lock,omitempty"`
	Documentation string `json:"documentation_url,omitempty"`
	RequestID     string `json:"request_id,omitempty"`
}

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
	ctx.JSON(404, LockError{Message: "Not found"})
}

// PostLockHandler create lock
func PostLockHandler(ctx *context.Context) {
	status := checkRequest(ctx.Req)
	if status != 200 {
		writeStatus(ctx, status)
		return
	}
	ctx.Resp.Header().Set("Content-Type", metaMediaType)
	ctx.JSON(404, LockError{Message: "Not found"})
}

// VerifyLockHandler list locks for verification
func VerifyLockHandler(ctx *context.Context) {
	status := checkRequest(ctx.Req)
	if status != 200 {
		writeStatus(ctx, status)
		return
	}
	ctx.Resp.Header().Set("Content-Type", metaMediaType)
	ctx.JSON(404, LockError{Message: "Not found"})
}

// UnLockHandler delete locks
func UnLockHandler(ctx *context.Context) {
	status := checkRequest(ctx.Req)
	if status != 200 {
		writeStatus(ctx, status)
		return
	}
	ctx.Resp.Header().Set("Content-Type", metaMediaType)
	ctx.JSON(404, LockError{Message: "Not found"})
}
