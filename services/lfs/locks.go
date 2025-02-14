// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"net/http"
	"strconv"
	"strings"

	auth_model "code.gitea.io/gitea/models/auth"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/json"
	lfs_module "code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

func handleLockListOut(ctx *context.Context, repo *repo_model.Repository, lock *git_model.LFSLock, err error) {
	if err != nil {
		if git_model.IsErrLFSLockNotExist(err) {
			ctx.JSON(http.StatusOK, api.LFSLockList{
				Locks: []*api.LFSLock{},
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, api.LFSLockError{
			Message: "unable to list locks : Internal Server Error",
		})
		return
	}
	if repo.ID != lock.RepoID {
		ctx.JSON(http.StatusOK, api.LFSLockList{
			Locks: []*api.LFSLock{},
		})
		return
	}
	ctx.JSON(http.StatusOK, api.LFSLockList{
		Locks: []*api.LFSLock{convert.ToLFSLock(ctx, lock)},
	})
}

// GetListLockHandler list locks
func GetListLockHandler(ctx *context.Context) {
	rv := getRequestContext(ctx)

	repository, err := repo_model.GetRepositoryByOwnerAndName(ctx, rv.User, rv.Repo)
	if err != nil {
		log.Debug("Could not find repository: %s/%s - %s", rv.User, rv.Repo, err)
		ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="gitea-lfs"`)
		ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
			Message: "You must have pull access to list locks",
		})
		return
	}
	repository.MustOwner(ctx)

	context.CheckRepoScopedToken(ctx, repository, auth_model.Read)
	if ctx.Written() {
		return
	}

	authenticated := authenticate(ctx, repository, rv.Authorization, true, false)
	if !authenticated {
		ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="gitea-lfs"`)
		ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
			Message: "You must have pull access to list locks",
		})
		return
	}
	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	cursor := ctx.FormInt("cursor")
	if cursor < 0 {
		cursor = 0
	}
	limit := ctx.FormInt("limit")
	if limit > setting.LFS.LocksPagingNum && setting.LFS.LocksPagingNum > 0 {
		limit = setting.LFS.LocksPagingNum
	} else if limit < 0 {
		limit = 0
	}
	id := ctx.FormString("id")
	if id != "" { // Case where we request a specific id
		v, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, api.LFSLockError{
				Message: "bad request : " + err.Error(),
			})
			return
		}
		lock, err := git_model.GetLFSLockByID(ctx, v)
		if err != nil && !git_model.IsErrLFSLockNotExist(err) {
			log.Error("Unable to get lock with ID[%s]: Error: %v", v, err)
		}
		handleLockListOut(ctx, repository, lock, err)
		return
	}

	path := ctx.FormString("path")
	if path != "" { // Case where we request a specific id
		lock, err := git_model.GetLFSLock(ctx, repository, path)
		if err != nil && !git_model.IsErrLFSLockNotExist(err) {
			log.Error("Unable to get lock for repository %-v with path %s: Error: %v", repository, path, err)
		}
		handleLockListOut(ctx, repository, lock, err)
		return
	}

	// If no query params path or id
	lockList, err := git_model.GetLFSLockByRepoID(ctx, repository.ID, cursor, limit)
	if err != nil {
		log.Error("Unable to list locks for repository ID[%d]: Error: %v", repository.ID, err)
		ctx.JSON(http.StatusInternalServerError, api.LFSLockError{
			Message: "unable to list locks : Internal Server Error",
		})
		return
	}
	lockListAPI := make([]*api.LFSLock, len(lockList))
	next := ""
	for i, l := range lockList {
		lockListAPI[i] = convert.ToLFSLock(ctx, l)
	}
	if limit > 0 && len(lockList) == limit {
		next = strconv.Itoa(cursor + 1)
	}
	ctx.JSON(http.StatusOK, api.LFSLockList{
		Locks: lockListAPI,
		Next:  next,
	})
}

// PostLockHandler create lock
func PostLockHandler(ctx *context.Context) {
	userName := ctx.PathParam("username")
	repoName := strings.TrimSuffix(ctx.PathParam("reponame"), ".git")
	authorization := ctx.Req.Header.Get("Authorization")

	repository, err := repo_model.GetRepositoryByOwnerAndName(ctx, userName, repoName)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", userName, repoName, err)
		ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="gitea-lfs"`)
		ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
			Message: "You must have push access to create locks",
		})
		return
	}
	repository.MustOwner(ctx)

	context.CheckRepoScopedToken(ctx, repository, auth_model.Write)
	if ctx.Written() {
		return
	}

	authenticated := authenticate(ctx, repository, authorization, true, true)
	if !authenticated {
		ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="gitea-lfs"`)
		ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
			Message: "You must have push access to create locks",
		})
		return
	}

	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	var req api.LFSLockRequest
	bodyReader := ctx.Req.Body
	defer bodyReader.Close()

	dec := json.NewDecoder(bodyReader)
	if err := dec.Decode(&req); err != nil {
		log.Warn("Failed to decode lock request as json. Error: %v", err)
		writeStatus(ctx, http.StatusBadRequest)
		return
	}

	lock, err := git_model.CreateLFSLock(ctx, repository, &git_model.LFSLock{
		Path:    req.Path,
		OwnerID: ctx.Doer.ID,
	})
	if err != nil {
		if git_model.IsErrLFSLockAlreadyExist(err) {
			ctx.JSON(http.StatusConflict, api.LFSLockError{
				Lock:    convert.ToLFSLock(ctx, lock),
				Message: "already created lock",
			})
			return
		}
		if git_model.IsErrLFSUnauthorizedAction(err) {
			ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="gitea-lfs"`)
			ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
				Message: "You must have push access to create locks : " + err.Error(),
			})
			return
		}
		log.Error("Unable to CreateLFSLock in repository %-v at %s for user %-v: Error: %v", repository, req.Path, ctx.Doer, err)
		ctx.JSON(http.StatusInternalServerError, api.LFSLockError{
			Message: "internal server error : Internal Server Error",
		})
		return
	}
	ctx.JSON(http.StatusCreated, api.LFSLockResponse{Lock: convert.ToLFSLock(ctx, lock)})
}

// VerifyLockHandler list locks for verification
func VerifyLockHandler(ctx *context.Context) {
	userName := ctx.PathParam("username")
	repoName := strings.TrimSuffix(ctx.PathParam("reponame"), ".git")
	authorization := ctx.Req.Header.Get("Authorization")

	repository, err := repo_model.GetRepositoryByOwnerAndName(ctx, userName, repoName)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", userName, repoName, err)
		ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="gitea-lfs"`)
		ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
			Message: "You must have push access to verify locks",
		})
		return
	}
	repository.MustOwner(ctx)

	context.CheckRepoScopedToken(ctx, repository, auth_model.Read)
	if ctx.Written() {
		return
	}

	authenticated := authenticate(ctx, repository, authorization, true, true)
	if !authenticated {
		ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="gitea-lfs"`)
		ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
			Message: "You must have push access to verify locks",
		})
		return
	}

	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	cursor := ctx.FormInt("cursor")
	if cursor < 0 {
		cursor = 0
	}
	limit := ctx.FormInt("limit")
	if limit > setting.LFS.LocksPagingNum && setting.LFS.LocksPagingNum > 0 {
		limit = setting.LFS.LocksPagingNum
	} else if limit < 0 {
		limit = 0
	}
	lockList, err := git_model.GetLFSLockByRepoID(ctx, repository.ID, cursor, limit)
	if err != nil {
		log.Error("Unable to list locks for repository ID[%d]: Error: %v", repository.ID, err)
		ctx.JSON(http.StatusInternalServerError, api.LFSLockError{
			Message: "unable to list locks : Internal Server Error",
		})
		return
	}
	next := ""
	if limit > 0 && len(lockList) == limit {
		next = strconv.Itoa(cursor + 1)
	}
	lockOursListAPI := make([]*api.LFSLock, 0, len(lockList))
	lockTheirsListAPI := make([]*api.LFSLock, 0, len(lockList))
	for _, l := range lockList {
		if l.OwnerID == ctx.Doer.ID {
			lockOursListAPI = append(lockOursListAPI, convert.ToLFSLock(ctx, l))
		} else {
			lockTheirsListAPI = append(lockTheirsListAPI, convert.ToLFSLock(ctx, l))
		}
	}
	ctx.JSON(http.StatusOK, api.LFSLockListVerify{
		Ours:   lockOursListAPI,
		Theirs: lockTheirsListAPI,
		Next:   next,
	})
}

// UnLockHandler delete locks
func UnLockHandler(ctx *context.Context) {
	userName := ctx.PathParam("username")
	repoName := strings.TrimSuffix(ctx.PathParam("reponame"), ".git")
	authorization := ctx.Req.Header.Get("Authorization")

	repository, err := repo_model.GetRepositoryByOwnerAndName(ctx, userName, repoName)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", userName, repoName, err)
		ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="gitea-lfs"`)
		ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
			Message: "You must have push access to delete locks",
		})
		return
	}
	repository.MustOwner(ctx)

	context.CheckRepoScopedToken(ctx, repository, auth_model.Write)
	if ctx.Written() {
		return
	}

	authenticated := authenticate(ctx, repository, authorization, true, true)
	if !authenticated {
		ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="gitea-lfs"`)
		ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
			Message: "You must have push access to delete locks",
		})
		return
	}

	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	var req api.LFSLockDeleteRequest
	bodyReader := ctx.Req.Body
	defer bodyReader.Close()

	dec := json.NewDecoder(bodyReader)
	if err := dec.Decode(&req); err != nil {
		log.Warn("Failed to decode lock request as json. Error: %v", err)
		writeStatus(ctx, http.StatusBadRequest)
		return
	}

	lock, err := git_model.DeleteLFSLockByID(ctx, ctx.PathParamInt64("lid"), repository, ctx.Doer, req.Force)
	if err != nil {
		if git_model.IsErrLFSUnauthorizedAction(err) {
			ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="gitea-lfs"`)
			ctx.JSON(http.StatusUnauthorized, api.LFSLockError{
				Message: "You must have push access to delete locks : " + err.Error(),
			})
			return
		}
		log.Error("Unable to DeleteLFSLockByID[%d] by user %-v with force %t: Error: %v", ctx.PathParamInt64("lid"), ctx.Doer, req.Force, err)
		ctx.JSON(http.StatusInternalServerError, api.LFSLockError{
			Message: "unable to delete lock : Internal Server Error",
		})
		return
	}
	ctx.JSON(http.StatusOK, api.LFSLockResponse{Lock: convert.ToLFSLock(ctx, lock)})
}
