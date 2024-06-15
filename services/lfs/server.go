// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	stdCtx "context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	lfs_module "code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/services/context"

	"github.com/golang-jwt/jwt/v5"
)

// requestContext contain variables from the HTTP request.
type requestContext struct {
	User          string
	Repo          string
	Authorization string
}

// Claims is a JWT Token Claims
type Claims struct {
	RepoID int64
	Op     string
	UserID int64
	jwt.RegisteredClaims
}

// DownloadLink builds a URL to download the object.
func (rc *requestContext) DownloadLink(p lfs_module.Pointer) string {
	return setting.AppURL + path.Join(url.PathEscape(rc.User), url.PathEscape(rc.Repo+".git"), "info/lfs/objects", url.PathEscape(p.Oid))
}

// UploadLink builds a URL to upload the object.
func (rc *requestContext) UploadLink(p lfs_module.Pointer) string {
	return setting.AppURL + path.Join(url.PathEscape(rc.User), url.PathEscape(rc.Repo+".git"), "info/lfs/objects", url.PathEscape(p.Oid), strconv.FormatInt(p.Size, 10))
}

// VerifyLink builds a URL for verifying the object.
func (rc *requestContext) VerifyLink(p lfs_module.Pointer) string {
	return setting.AppURL + path.Join(url.PathEscape(rc.User), url.PathEscape(rc.Repo+".git"), "info/lfs/verify")
}

// CheckAcceptMediaType checks if the client accepts the LFS media type.
func CheckAcceptMediaType(ctx *context.Context) {
	mediaParts := strings.Split(ctx.Req.Header.Get("Accept"), ";")

	if mediaParts[0] != lfs_module.MediaType {
		log.Trace("Calling a LFS method without accepting the correct media type: %s", lfs_module.MediaType)
		writeStatus(ctx, http.StatusUnsupportedMediaType)
		return
	}
}

var rangeHeaderRegexp = regexp.MustCompile(`bytes=(\d+)\-(\d*).*`)

// DownloadHandler gets the content from the content store
func DownloadHandler(ctx *context.Context) {
	rc := getRequestContext(ctx)
	p := lfs_module.Pointer{Oid: ctx.Params("oid")}

	meta := getAuthenticatedMeta(ctx, rc, p, false)
	if meta == nil {
		return
	}

	// Support resume download using Range header
	var fromByte, toByte int64
	toByte = meta.Size - 1
	statusCode := http.StatusOK
	if rangeHdr := ctx.Req.Header.Get("Range"); rangeHdr != "" {
		match := rangeHeaderRegexp.FindStringSubmatch(rangeHdr)
		if len(match) > 1 {
			statusCode = http.StatusPartialContent
			fromByte, _ = strconv.ParseInt(match[1], 10, 32)

			if fromByte >= meta.Size {
				writeStatus(ctx, http.StatusRequestedRangeNotSatisfiable)
				return
			}

			if match[2] != "" {
				_toByte, _ := strconv.ParseInt(match[2], 10, 32)
				if _toByte >= fromByte && _toByte < toByte {
					toByte = _toByte
				}
			}

			ctx.Resp.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", fromByte, toByte, meta.Size-fromByte))
			ctx.Resp.Header().Set("Access-Control-Expose-Headers", "Content-Range")
		}
	}

	contentStore := lfs_module.NewContentStore()
	content, err := contentStore.Get(meta.Pointer)
	if err != nil {
		writeStatus(ctx, http.StatusNotFound)
		return
	}
	defer content.Close()

	if fromByte > 0 {
		_, err = content.Seek(fromByte, io.SeekStart)
		if err != nil {
			log.Error("Whilst trying to read LFS OID[%s]: Unable to seek to %d Error: %v", meta.Oid, fromByte, err)
			writeStatus(ctx, http.StatusInternalServerError)
			return
		}
	}

	contentLength := toByte + 1 - fromByte
	ctx.Resp.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	ctx.Resp.Header().Set("Content-Type", "application/octet-stream")

	filename := ctx.Params("filename")
	if len(filename) > 0 {
		decodedFilename, err := base64.RawURLEncoding.DecodeString(filename)
		if err == nil {
			ctx.Resp.Header().Set("Content-Disposition", "attachment; filename=\""+string(decodedFilename)+"\"")
			ctx.Resp.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
		}
	}

	ctx.Resp.WriteHeader(statusCode)
	if written, err := io.CopyN(ctx.Resp, content, contentLength); err != nil {
		log.Error("Error whilst copying LFS OID[%s] to the response after %d bytes. Error: %v", meta.Oid, written, err)
	}
}

// BatchHandler provides the batch api
func BatchHandler(ctx *context.Context) {
	var br lfs_module.BatchRequest
	if err := decodeJSON(ctx.Req, &br); err != nil {
		log.Trace("Unable to decode BATCH request vars: Error: %v", err)
		writeStatus(ctx, http.StatusBadRequest)
		return
	}

	var isUpload bool
	if br.Operation == "upload" {
		isUpload = true
	} else if br.Operation == "download" {
		isUpload = false
	} else {
		log.Trace("Attempt to BATCH with invalid operation: %s", br.Operation)
		writeStatus(ctx, http.StatusBadRequest)
		return
	}

	rc := getRequestContext(ctx)

	repository := getAuthenticatedRepository(ctx, rc, isUpload)
	if repository == nil {
		return
	}

	contentStore := lfs_module.NewContentStore()

	var responseObjects []*lfs_module.ObjectResponse

	for _, p := range br.Objects {
		if !p.IsValid() {
			responseObjects = append(responseObjects, buildObjectResponse(rc, p, false, false, &lfs_module.ObjectError{
				Code:    http.StatusUnprocessableEntity,
				Message: "Oid or size are invalid",
			}))
			continue
		}

		exists, err := contentStore.Exists(p)
		if err != nil {
			log.Error("Unable to check if LFS OID[%s] exist. Error: %v", p.Oid, rc.User, rc.Repo, err)
			writeStatus(ctx, http.StatusInternalServerError)
			return
		}

		meta, err := git_model.GetLFSMetaObjectByOid(ctx, repository.ID, p.Oid)
		if err != nil && err != git_model.ErrLFSObjectNotExist {
			log.Error("Unable to get LFS MetaObject [%s] for %s/%s. Error: %v", p.Oid, rc.User, rc.Repo, err)
			writeStatus(ctx, http.StatusInternalServerError)
			return
		}

		if meta != nil && p.Size != meta.Size {
			responseObjects = append(responseObjects, buildObjectResponse(rc, p, false, false, &lfs_module.ObjectError{
				Code:    http.StatusUnprocessableEntity,
				Message: fmt.Sprintf("Object %s is not %d bytes", p.Oid, p.Size),
			}))
			continue
		}

		var responseObject *lfs_module.ObjectResponse
		if isUpload {
			var err *lfs_module.ObjectError
			if !exists && setting.LFS.MaxFileSize > 0 && p.Size > setting.LFS.MaxFileSize {
				err = &lfs_module.ObjectError{
					Code:    http.StatusUnprocessableEntity,
					Message: fmt.Sprintf("Size must be less than or equal to %d", setting.LFS.MaxFileSize),
				}
			}

			if exists && meta == nil {
				accessible, err := git_model.LFSObjectAccessible(ctx, ctx.Doer, p.Oid)
				if err != nil {
					log.Error("Unable to check if LFS MetaObject [%s] is accessible. Error: %v", p.Oid, err)
					writeStatus(ctx, http.StatusInternalServerError)
					return
				}
				if accessible {
					_, err := git_model.NewLFSMetaObject(ctx, repository.ID, p)
					if err != nil {
						log.Error("Unable to create LFS MetaObject [%s] for %s/%s. Error: %v", p.Oid, rc.User, rc.Repo, err)
						writeStatus(ctx, http.StatusInternalServerError)
						return
					}
				} else {
					exists = false
				}
			}

			responseObject = buildObjectResponse(rc, p, false, !exists, err)
		} else {
			var err *lfs_module.ObjectError
			if !exists || meta == nil {
				err = &lfs_module.ObjectError{
					Code:    http.StatusNotFound,
					Message: http.StatusText(http.StatusNotFound),
				}
			}

			responseObject = buildObjectResponse(rc, p, true, false, err)
		}
		responseObjects = append(responseObjects, responseObject)
	}

	respobj := &lfs_module.BatchResponse{Objects: responseObjects}

	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	enc := json.NewEncoder(ctx.Resp)
	if err := enc.Encode(respobj); err != nil {
		log.Error("Failed to encode representation as json. Error: %v", err)
	}
}

// UploadHandler receives data from the client and puts it into the content store
func UploadHandler(ctx *context.Context) {
	rc := getRequestContext(ctx)

	p := lfs_module.Pointer{Oid: ctx.Params("oid")}
	var err error
	if p.Size, err = strconv.ParseInt(ctx.Params("size"), 10, 64); err != nil {
		writeStatusMessage(ctx, http.StatusUnprocessableEntity, err.Error())
	}

	if !p.IsValid() {
		log.Trace("Attempt to access invalid LFS OID[%s] in %s/%s", p.Oid, rc.User, rc.Repo)
		writeStatus(ctx, http.StatusUnprocessableEntity)
		return
	}

	repository := getAuthenticatedRepository(ctx, rc, true)
	if repository == nil {
		return
	}

	contentStore := lfs_module.NewContentStore()
	exists, err := contentStore.Exists(p)
	if err != nil {
		log.Error("Unable to check if LFS OID[%s] exist. Error: %v", p.Oid, err)
		writeStatus(ctx, http.StatusInternalServerError)
		return
	}

	uploadOrVerify := func() error {
		if exists {
			accessible, err := git_model.LFSObjectAccessible(ctx, ctx.Doer, p.Oid)
			if err != nil {
				log.Error("Unable to check if LFS MetaObject [%s] is accessible. Error: %v", p.Oid, err)
				return err
			}
			if !accessible {
				// The file exists but the user has no access to it.
				// The upload gets verified by hashing and size comparison to prove access to it.
				hash := sha256.New()
				written, err := io.Copy(hash, ctx.Req.Body)
				if err != nil {
					log.Error("Error creating hash. Error: %v", err)
					return err
				}

				if written != p.Size {
					return lfs_module.ErrSizeMismatch
				}
				if hex.EncodeToString(hash.Sum(nil)) != p.Oid {
					return lfs_module.ErrHashMismatch
				}
			}
		} else if err := contentStore.Put(p, ctx.Req.Body); err != nil {
			log.Error("Error putting LFS MetaObject [%s] into content store. Error: %v", p.Oid, err)
			return err
		}
		_, err := git_model.NewLFSMetaObject(ctx, repository.ID, p)
		return err
	}

	defer ctx.Req.Body.Close()
	if err := uploadOrVerify(); err != nil {
		if errors.Is(err, lfs_module.ErrSizeMismatch) || errors.Is(err, lfs_module.ErrHashMismatch) {
			log.Error("Upload does not match LFS MetaObject [%s]. Error: %v", p.Oid, err)
			writeStatusMessage(ctx, http.StatusUnprocessableEntity, err.Error())
		} else {
			log.Error("Error whilst uploadOrVerify LFS OID[%s]: %v", p.Oid, err)
			writeStatus(ctx, http.StatusInternalServerError)
		}
		if _, err = git_model.RemoveLFSMetaObjectByOid(ctx, repository.ID, p.Oid); err != nil {
			log.Error("Error whilst removing MetaObject for LFS OID[%s]: %v", p.Oid, err)
		}
		return
	}

	writeStatus(ctx, http.StatusOK)
}

// VerifyHandler verify oid and its size from the content store
func VerifyHandler(ctx *context.Context) {
	var p lfs_module.Pointer
	if err := decodeJSON(ctx.Req, &p); err != nil {
		writeStatus(ctx, http.StatusUnprocessableEntity)
		return
	}

	rc := getRequestContext(ctx)

	meta := getAuthenticatedMeta(ctx, rc, p, true)
	if meta == nil {
		return
	}

	contentStore := lfs_module.NewContentStore()
	ok, err := contentStore.Verify(meta.Pointer)

	status := http.StatusOK
	if err != nil {
		log.Error("Error whilst verifying LFS OID[%s]: %v", p.Oid, err)
		status = http.StatusInternalServerError
	} else if !ok {
		status = http.StatusNotFound
	}
	writeStatus(ctx, status)
}

func decodeJSON(req *http.Request, v any) error {
	defer req.Body.Close()

	dec := json.NewDecoder(req.Body)
	return dec.Decode(v)
}

func getRequestContext(ctx *context.Context) *requestContext {
	return &requestContext{
		User:          ctx.Params("username"),
		Repo:          strings.TrimSuffix(ctx.Params("reponame"), ".git"),
		Authorization: ctx.Req.Header.Get("Authorization"),
	}
}

func getAuthenticatedMeta(ctx *context.Context, rc *requestContext, p lfs_module.Pointer, requireWrite bool) *git_model.LFSMetaObject {
	if !p.IsValid() {
		log.Info("Attempt to access invalid LFS OID[%s] in %s/%s", p.Oid, rc.User, rc.Repo)
		writeStatusMessage(ctx, http.StatusUnprocessableEntity, "Oid or size are invalid")
		return nil
	}

	repository := getAuthenticatedRepository(ctx, rc, requireWrite)
	if repository == nil {
		return nil
	}

	meta, err := git_model.GetLFSMetaObjectByOid(ctx, repository.ID, p.Oid)
	if err != nil {
		log.Error("Unable to get LFS OID[%s] Error: %v", p.Oid, err)
		writeStatus(ctx, http.StatusNotFound)
		return nil
	}

	return meta
}

func getAuthenticatedRepository(ctx *context.Context, rc *requestContext, requireWrite bool) *repo_model.Repository {
	repository, err := repo_model.GetRepositoryByOwnerAndName(ctx, rc.User, rc.Repo)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", rc.User, rc.Repo, err)
		writeStatus(ctx, http.StatusNotFound)
		return nil
	}

	if !authenticate(ctx, repository, rc.Authorization, false, requireWrite) {
		requireAuth(ctx)
		return nil
	}

	if requireWrite {
		context.CheckRepoScopedToken(ctx, repository, auth_model.Write)
	} else {
		context.CheckRepoScopedToken(ctx, repository, auth_model.Read)
	}

	if ctx.Written() {
		return nil
	}

	return repository
}

func buildObjectResponse(rc *requestContext, pointer lfs_module.Pointer, download, upload bool, err *lfs_module.ObjectError) *lfs_module.ObjectResponse {
	rep := &lfs_module.ObjectResponse{Pointer: pointer}
	if err != nil {
		rep.Error = err
	} else {
		rep.Actions = make(map[string]*lfs_module.Link)

		header := make(map[string]string)

		if len(rc.Authorization) > 0 {
			header["Authorization"] = rc.Authorization
		}

		if download {
			var link *lfs_module.Link
			if setting.LFS.Storage.ServeDirect() {
				// If we have a signed url (S3, object storage), redirect to this directly.
				u, err := storage.LFS.URL(pointer.RelativePath(), pointer.Oid)
				if u != nil && err == nil {
					// Presigned url does not need the Authorization header
					// https://github.com/go-gitea/gitea/issues/21525
					delete(header, "Authorization")
					link = &lfs_module.Link{Href: u.String(), Header: header}
				}
			}
			if link == nil {
				link = &lfs_module.Link{Href: rc.DownloadLink(pointer), Header: header}
			}
			rep.Actions["download"] = link
		}
		if upload {
			rep.Actions["upload"] = &lfs_module.Link{Href: rc.UploadLink(pointer), Header: header}

			verifyHeader := make(map[string]string)
			for key, value := range header {
				verifyHeader[key] = value
			}

			// This is only needed to workaround https://github.com/git-lfs/git-lfs/issues/3662
			verifyHeader["Accept"] = lfs_module.AcceptHeader

			rep.Actions["verify"] = &lfs_module.Link{Href: rc.VerifyLink(pointer), Header: verifyHeader}
		}
	}
	return rep
}

func writeStatus(ctx *context.Context, status int) {
	writeStatusMessage(ctx, status, http.StatusText(status))
}

func writeStatusMessage(ctx *context.Context, status int, message string) {
	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)
	ctx.Resp.WriteHeader(status)

	er := lfs_module.ErrorResponse{Message: message}

	enc := json.NewEncoder(ctx.Resp)
	if err := enc.Encode(er); err != nil {
		log.Error("Failed to encode error response as json. Error: %v", err)
	}
}

// authenticate uses the authorization string to determine whether
// or not to proceed. This server assumes an HTTP Basic auth format.
func authenticate(ctx *context.Context, repository *repo_model.Repository, authorization string, requireSigned, requireWrite bool) bool {
	accessMode := perm.AccessModeRead
	if requireWrite {
		accessMode = perm.AccessModeWrite
	}

	if ctx.Data["IsActionsToken"] == true {
		taskID := ctx.Data["ActionsTaskID"].(int64)
		task, err := actions_model.GetTaskByID(ctx, taskID)
		if err != nil {
			log.Error("Unable to GetTaskByID for task[%d] Error: %v", taskID, err)
			return false
		}
		if task.RepoID != repository.ID {
			return false
		}

		if task.IsForkPullRequest {
			return accessMode <= perm.AccessModeRead
		}
		return accessMode <= perm.AccessModeWrite
	}

	// ctx.IsSigned is unnecessary here, this will be checked in perm.CanAccess
	perm, err := access_model.GetUserRepoPermission(ctx, repository, ctx.Doer)
	if err != nil {
		log.Error("Unable to GetUserRepoPermission for user %-v in repo %-v Error: %v", ctx.Doer, repository, err)
		return false
	}

	canRead := perm.CanAccess(accessMode, unit.TypeCode)
	if canRead && (!requireSigned || ctx.IsSigned) {
		return true
	}

	user, err := parseToken(ctx, authorization, repository, accessMode)
	if err != nil {
		// Most of these are Warn level - the true internal server errors are logged in parseToken already
		log.Warn("Authentication failure for provided token with Error: %v", err)
		return false
	}
	ctx.Doer = user
	return true
}

func handleLFSToken(ctx stdCtx.Context, tokenSHA string, target *repo_model.Repository, mode perm.AccessMode) (*user_model.User, error) {
	if !strings.Contains(tokenSHA, ".") {
		return nil, nil
	}
	token, err := jwt.ParseWithClaims(tokenSHA, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return setting.LFS.JWTSecretBytes, nil
	})
	if err != nil {
		return nil, nil
	}

	claims, claimsOk := token.Claims.(*Claims)
	if !token.Valid || !claimsOk {
		return nil, fmt.Errorf("invalid token claim")
	}

	if claims.RepoID != target.ID {
		return nil, fmt.Errorf("invalid token claim")
	}

	if mode == perm.AccessModeWrite && claims.Op != "upload" {
		return nil, fmt.Errorf("invalid token claim")
	}

	u, err := user_model.GetUserByID(ctx, claims.UserID)
	if err != nil {
		log.Error("Unable to GetUserById[%d]: Error: %v", claims.UserID, err)
		return nil, err
	}
	return u, nil
}

func parseToken(ctx stdCtx.Context, authorization string, target *repo_model.Repository, mode perm.AccessMode) (*user_model.User, error) {
	if authorization == "" {
		return nil, fmt.Errorf("no token")
	}

	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("no token")
	}
	tokenSHA := parts[1]
	switch strings.ToLower(parts[0]) {
	case "bearer":
		fallthrough
	case "token":
		return handleLFSToken(ctx, tokenSHA, target, mode)
	}
	return nil, fmt.Errorf("token not found")
}

func requireAuth(ctx *context.Context) {
	ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
	writeStatus(ctx, http.StatusUnauthorized)
}
