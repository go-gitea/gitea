// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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

func checkRequest(req macaron.Request) int {
	if !setting.LFS.StartServer {
		return 404
	}
	if !MetaMatcher(req) || req.Header.Get("Content-Type") != metaMediaType {
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
	status := checkRequest(ctx.Req)
	if status != 200 {
		writeStatus(ctx, status)
		return
	}
	ctx.Resp.Header().Set("Content-Type", metaMediaType)

	err := models.CheckLFSAccessForRepo(ctx.User, ctx.Repo.Repository.ID, "list")
	if err != nil {
		if models.IsErrLFSLockUnauthorizedAction(err) {
			ctx.JSON(403, api.LFSLockError{
				Message: "You must have pull access to list locks : " + err.Error(),
			})
			return
		}
		ctx.JSON(500, api.LFSLockError{
			Message: "unable to list lock : " + err.Error(),
		})
		return
	}
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
		Owner:  ctx.User,
	})
	if err != nil {
		if models.IsErrLFSLockAlreadyExist(err) {
			ctx.JSON(409, api.LFSLockError{
				Lock:    lock.APIFormat(),
				Message: "already created lock",
			})
			return
		}
		if models.IsErrLFSLockUnauthorizedAction(err) {
			ctx.JSON(403, api.LFSLockError{
				Message: "You must have push access to create locks : " + err.Error(),
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
	status := checkRequest(ctx.Req)
	if status != 200 {
		writeStatus(ctx, status)
		return
	}

	ctx.Resp.Header().Set("Content-Type", metaMediaType)

	err := models.CheckLFSAccessForRepo(ctx.User, ctx.Repo.Repository.ID, "verify")
	if err != nil {
		if models.IsErrLFSLockUnauthorizedAction(err) {
			ctx.JSON(403, api.LFSLockError{
				Message: "You must have push access to verify locks : " + err.Error(),
			})
			return
		}
		ctx.JSON(500, api.LFSLockError{
			Message: "unable to verify lock : " + err.Error(),
		})
		return
	}

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
	for _, l := range lockList {
		if l.Owner.ID == ctx.User.ID {
			lockOursListAPI = append(lockOursListAPI, l.APIFormat())
		} else {
			lockTheirsListAPI = append(lockTheirsListAPI, l.APIFormat())
		}
	}
	ctx.JSON(200, api.LFSLockListVerify{
		Ours:   lockOursListAPI,
		Theirs: lockTheirsListAPI,
	})
}

// UnLockHandler delete locks
func UnLockHandler(ctx *context.Context) {
	status := checkRequest(ctx.Req)
	if status != 200 {
		writeStatus(ctx, status)
		return
	}
	ctx.Resp.Header().Set("Content-Type", metaMediaType)

	var req api.LFSLockDeleteRequest
	dec := json.NewDecoder(ctx.Req.Body().ReadCloser())
	err := dec.Decode(&req)
	if err != nil {
		writeStatus(ctx, 400)
		return
	}

	lock, err := models.DeleteLFSLockByID(ctx.ParamsInt64("lid"), ctx.User, req.Force)
	if err != nil {
		if models.IsErrLFSLockUnauthorizedAction(err) {
			ctx.JSON(403, api.LFSLockError{
				Message: "You must have push access to delete locks : " + err.Error(),
			})
			return
		}
		ctx.JSON(500, api.LFSLockError{
			Message: "unable to delete lock : " + err.Error(),
		})
		return
	}
	ctx.JSON(200, api.LFSLockResponse{Lock: lock.APIFormat()})
}
