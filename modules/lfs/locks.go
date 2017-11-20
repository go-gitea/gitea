package lfs

import (
	"encoding/json"
	"strconv"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/sdk/gitea"

	"gopkg.in/macaron.v1"
)

//TODO handle 403 forbidden

func checkRequest(req macaron.Request) int {
	if !setting.LFS.StartServer {
		return 404
	}
	if !ContentMatcher(req) || req.Header.Get("Content-Type") != contentMediaType {
		return 400
	}
	return 200
}

func handleLockListOut(ctx *context.Context, lock *models.LFSLock, err error) {
	if err != nil {
		if models.IsErrLFSLockNotExist(err) {
			ctx.JSON(200, api.LFSLockList{
				Locks: []*api.LFSLock{},
			})
			return
		}
		ctx.JSON(500, api.LFSLockError{
			Message: "unable to list locks : " + err.Error(),
		})
		return
	}
	if ctx.Repo.Repository.ID != lock.RepoID {
		ctx.JSON(200, api.LFSLockList{
			Locks: []*api.LFSLock{},
		})
		return
	}
	ctx.JSON(200, api.LFSLockList{
		Locks: []*api.LFSLock{lock.APIFormat()},
	})
}

// GetListLockHandler list locks
func GetListLockHandler(ctx *context.Context) {
	//TODO LFS Servers should ensure that users have at least pull access to the repository
	status := checkRequest(ctx.Req)
	if status != 200 {
		writeStatus(ctx, status)
		return
	}
	ctx.Resp.Header().Set("Content-Type", metaMediaType)

	//TODO handle query cursor and limit

	id := ctx.Query("id")
	if id != "" { //Case where we request a specific id
		v, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			ctx.JSON(400, api.LFSLockError{
				Message: "bad request : " + err.Error(),
			})
			return
		}
		lock, err := models.GetLFSLockByID(int64(v))
		handleLockListOut(ctx, lock, err)
		return
	}

	path := ctx.Query("path")
	if path != "" { //Case where we request a specific id
		lock, err := models.GetLFSLock(ctx.Repo.Repository.ID, path)
		handleLockListOut(ctx, lock, err)
		return
	}

	//If no query params path or id
	lockList, err := models.GetLFSLockByRepoID(ctx.Repo.Repository.ID)
	if err != nil {
		ctx.JSON(500, api.LFSLockError{
			Message: "unable to list locks : " + err.Error(),
		})
		return
	}
	lockListAPI := make([]*api.LFSLock, len(lockList))
	for i, l := range lockList {
		lockListAPI[i] = l.APIFormat()
	}
	ctx.JSON(200, api.LFSLockList{
		Locks: lockListAPI,
	})
}

// PostLockHandler create lock
func PostLockHandler(ctx *context.Context) {
	//TODO Servers should ensure that users have push access to the repository, and that files are locked exclusively to one user.
	status := checkRequest(ctx.Req)
	if status != 200 {
		writeStatus(ctx, status)
		return
	}
	ctx.Resp.Header().Set("Content-Type", metaMediaType)

	var req api.LFSLockRequest
	dec := json.NewDecoder(ctx.Req.Body().ReadCloser())
	err := dec.Decode(&req)
	if err != nil {
		writeStatus(ctx, 400)
		return
	}

	lock, err := models.CreateLFSLock(&models.LFSLock{
		RepoID: ctx.Repo.Repository.ID,
		Path:   req.Path,
	})
	if err != nil {
		if models.IsErrLFSLockAlreadyExist(err) {
			ctx.JSON(409, api.LFSLockError{
				Lock:    lock.APIFormat(),
				Message: "already created lock",
			})
			return
		}
		ctx.JSON(500, api.LFSLockError{
			Message: "internal server error : " + err.Error(),
		})
		return
	}
	ctx.JSON(201, api.LFSLockResponse{Lock: lock.APIFormat()})
}

// VerifyLockHandler list locks for verification
func VerifyLockHandler(ctx *context.Context) {
	//TODO LFS Servers should ensure that users have push access to the repository.
	status := checkRequest(ctx.Req)
	if status != 200 {
		writeStatus(ctx, status)
		return
	}
	ctx.Resp.Header().Set("Content-Type", metaMediaType)
	//TODO handle body json cursor and limit
	lockList, err := models.GetLFSLockByRepoID(ctx.Repo.Repository.ID)
	if err != nil {
		ctx.JSON(500, api.LFSLockError{
			Message: "unable to list locks : " + err.Error(),
		})
		return
	}
	lockOursListAPI := make([]*api.LFSLock, 0, len(lockList))
	lockTheirsListAPI := make([]*api.LFSLock, 0, len(lockList))
	for i, l := range lockList {
		if l.Owner.ID == ctx.User.ID {
			lockOursListAPI[i] = l.APIFormat()
		} else {
			lockTheirsListAPI[i] = l.APIFormat()
		}
	}
	ctx.JSON(200, api.LFSLockListVerify{
		Ours:   lockOursListAPI,
		Theirs: lockTheirsListAPI,
	})
}

// UnLockHandler delete locks
func UnLockHandler(ctx *context.Context) {
	//TODO LFS servers should ensure that callers have push access to the repository. They should also prevent a user from deleting another user's lock, unless the force property is given.
	status := checkRequest(ctx.Req)
	if status != 200 {
		writeStatus(ctx, status)
		return
	}
	ctx.Resp.Header().Set("Content-Type", metaMediaType)
	//TODO
	ctx.JSON(404, api.LFSLockError{Message: "Not found"})
}
