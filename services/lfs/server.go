// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	lfs_module "code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/dgrijalva/jwt-go"
	jsoniter "github.com/json-iterator/go"
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
	jwt.StandardClaims
}

// ObjectLink builds a URL linking to the object.
func (rc *requestContext) ObjectLink(oid string) string {
	return setting.AppURL + path.Join(rc.User, rc.Repo+".git", "info/lfs/objects", oid)
}

// VerifyLink builds a URL for verifying the object.
func (rc *requestContext) VerifyLink(oid string) string {
	return setting.AppURL + path.Join(rc.User, rc.Repo+".git", "info/lfs/verify", oid)
}

// CheckAcceptMediaType checks if the client accepts the LFS media type.
func CheckAcceptMediaType(ctx *context.Context) {
	mediaParts := strings.Split(r.Header.Get("Accept"), ";")

	if mediaParts[0] != lfs_module.MediaType {
		log.Info("Calling a LFS method without accepting the correct media type: %s", lfs_module.MediaType)
		writeStatus(ctx, http.StatusBadRequest)
		return
	}
}

func getAuthenticatedRepoAndMeta(ctx *context.Context, rc *requestContext, p lfs_module.Pointer, requireWrite bool) (*models.LFSMetaObject, *models.Repository) {
	if !p.IsValid() {
		log.Info("Attempt to access invalid LFS OID[%s] in %s/%s", p.Oid, rc.User, rc.Repo)
		writeStatus(ctx, http.StatusNotFound)
		return nil, nil
	}

	repository, err := models.GetRepositoryByOwnerAndName(rc.User, rc.Repo)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", rc.User, rc.Repo, err)
		writeStatus(ctx, http.StatusNotFound)
		return nil, nil
	}

	if !authenticate(ctx, repository, rc.Authorization, requireWrite) {
		requireAuth(ctx)
		return nil, nil
	}

	meta, err := repository.GetLFSMetaObjectByOid(p.Oid)
	if err != nil {
		log.Error("Unable to get LFS OID[%s] Error: %v", p.Oid, err)
		writeStatus(ctx, http.StatusNotFound)
		return nil, nil
	}

	return meta, repository
}

// DownloadHandler gets the content from the content store
func DownloadHandler(ctx *context.Context) {
	rc, p := unpack(ctx)

	meta, _ := getAuthenticatedRepoAndMeta(ctx, rc, p, false)
	if meta == nil {
		// Status already written in getAuthenticatedRepoAndMeta
		return
	}

	// Support resume download using Range header
	var fromByte, toByte int64
	toByte = meta.Size - 1
	statusCode := http.StatusOK
	if rangeHdr := ctx.Req.Header.Get("Range"); rangeHdr != "" {
		regex := regexp.MustCompile(`bytes=(\d+)\-(\d*).*`)
		match := regex.FindStringSubmatch(rangeHdr)
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
		// Errors are logged in contentStore.Get
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
	logRequest(ctx.Req, statusCode)
}

// LegacyMetaHandler retrieves metadata about the object
func LegacyMetaHandler(ctx *context.Context) {
	rc, p := unpack(ctx)

	meta, _ := getAuthenticatedRepoAndMeta(ctx, rc, p, false)
	if meta == nil {
		// Status already written in getAuthenticatedRepoAndMeta
		return
	}

	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	if ctx.Req.Method == "GET" {
		json := jsoniter.ConfigCompatibleWithStandardLibrary
		enc := json.NewEncoder(ctx.Resp)
		if err := enc.Encode(buildObjectResponse(rc, meta.Pointer, true, false, 0)); err != nil {
			log.Error("Failed to encode representation as json. Error: %v", err)
		}
	}

	logRequest(ctx.Req, http.StatusOK)
}

// LegacyPostHandler instructs the client how to upload data
func LegacyPostHandler(ctx *context.Context) {
	rc, p := unpack(ctx)

	repository, err := models.GetRepositoryByOwnerAndName(rc.User, rc.Repo)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", rc.User, rc.Repo, err)
		writeStatus(ctx, http.StatusNotFound)
		return
	}

	if !authenticate(ctx, repository, rc.Authorization, true) {
		requireAuth(ctx)
		return
	}

	if !p.IsValid() {
		log.Info("Invalid LFS OID[%s] attempt to POST in %s/%s", p.Oid, rc.User, rc.Repo)
		writeStatus(ctx, http.StatusNotFound)
		return
	}

	if setting.LFS.MaxFileSize > 0 && p.Size > setting.LFS.MaxFileSize {
		log.Info("Denied LFS OID[%s] upload of size %d to %s/%s because of LFS_MAX_FILE_SIZE=%d", p.Oid, p.Size, rc.User, rc.Repo, setting.LFS.MaxFileSize)
		writeStatus(ctx, http.StatusRequestEntityTooLarge)
		return
	}

	meta, err := models.NewLFSMetaObject(&models.LFSMetaObject{Pointer: p, RepositoryID: repository.ID})
	if err != nil {
		log.Error("Unable to write LFS OID[%s] size %d meta object in %v/%v to database. Error: %v", p.Oid, p.Size, rc.User, rc.Repo, err)
		writeStatus(ctx, http.StatusNotFound)
		return
	}

	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	sentStatus := http.StatusAccepted
	contentStore := lfs_module.NewContentStore()
	exist, err := contentStore.Exists(p)
	if err != nil {
		log.Error("Unable to check if LFS OID[%s] exist on %s / %s. Error: %v", p.Oid, rc.User, rc.Repo, err)
		writeStatus(ctx, http.StatusInternalServerError)
		return
	}
	if meta.Existing && exist {
		sentStatus = http.StatusOK
	}
	ctx.Resp.WriteHeader(sentStatus)

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	enc := json.NewEncoder(ctx.Resp)
	if err := enc.Encode(buildObjectResponse(rc, meta.Pointer, meta.Existing, true, 0)); err != nil {
		log.Error("Failed to encode representation as json. Error: %v", err)
	}
	logRequest(ctx.Req, sentStatus)
}

// BatchHandler provides the batch api
func BatchHandler(ctx *context.Context) {
	bv := unpackbatch(ctx)

	var isUpload bool
	if bv.Operation == "upload" {
		isUpload = true
	} else if bv.Operation == "download" {
		isUpload = false
	} else {
		log.Info("Attempt to BATCH with invalid operation: %s", bv.Operation)
		writeStatus(ctx, http.StatusBadRequest)
		return
	}

	reqCtx := getRequestContext(ctx)

	repository, err := models.GetRepositoryByOwnerAndName(reqCtx.User, reqCtx.Repo)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", reqCtx.User, reqCtx.Repo, err)
		writeStatus(ctx, http.StatusNotFound)
		return
	}

	if !authenticate(ctx, repository, reqCtx.Authorization, isUpload) {
		requireAuth(ctx)
		return
	}

	contentStore := lfs_module.NewContentStore()

	var responseObjects []*lfs_module.ObjectResponse

	for _, p := range bv.Objects {
		if !object.IsValid() {
			responseObjects = append(responseObjects, buildObjectResponse(reqCtx, p, false, false, http.StatusUnprocessableEntity))
			continue
		}

		exists, err := contentStore.Exists(p)
		if err != nil {
			log.Error("Unable to check if LFS OID[%s] exist on %s/%s. Error: %v", p.Oid, reqCtx.User, reqCtx.Repo, err)
			writeStatus(ctx, http.StatusInternalServerError)
			return
		}

		meta, metaErr := repository.GetLFSMetaObjectByOid(p.Oid)
		if metaErr != nil && metaErr != models.ErrLFSObjectNotExist {
			log.Error("Unable to get LFS MetaObject [%s] for %s/%s. Error: %v", p.Oid, reqCtx.User, reqCtx.Repo, metaErr)
			writeStatus(ctx, http.StatusInternalServerError)
			return
		}

		var responseObject *lfs_module.ObjectResponse
		if isUpload {
			if !exists && setting.LFS.MaxFileSize > 0 && object.Size > setting.LFS.MaxFileSize {
				log.Info("Denied LFS OID[%s] upload of size %d to %s/%s because of LFS_MAX_FILE_SIZE=%d", p.Oid, p.Size, reqCtx.User, reqCtx.Repo, setting.LFS.MaxFileSize)
				writeStatus(ctx, http.StatusRequestEntityTooLarge)
				return
			}

			if exists {
				if meta == nil {
					_, err := models.NewLFSMetaObject(&models.LFSMetaObject{Pointer: p, RepositoryID: repository.ID})
					if err != nil {
						log.Error("Unable to create LFS MetaObject [%s] for %s/%s. Error: %v", p.Oid, reqCtx.User, reqCtx.Repo, metaErr)
						writeStatus(ctx, http.StatusInternalServerError)
						return
					}
				}
			}

			responseObject = buildObjectResponse(reqCtx, p, false, !exists, 0)
		} else {
			errorCode := 0
			if !exists || meta == nil {
				errorCode = http.StatusNotFound
			} else if meta.Size != p.Size {
				errorCode = http.StatusUnprocessableEntity
			}

			responseObject = buildObjectResponse(reqCtx, p, true, false, errorCode)
		}
		responseObjects = append(responseObjects, responseObject)
	}

	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	respobj := &lfs_module.BatchResponse{Objects: responseObjects}

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	enc := json.NewEncoder(ctx.Resp)
	if err := enc.Encode(respobj); err != nil {
		log.Error("Failed to encode representation as json. Error: %v", err)
	}
	logRequest(ctx.Req, http.StatusOK)
}

// UploadHandler receives data from the client and puts it into the content store
func UploadHandler(ctx *context.Context) {
	rc, p := unpack(ctx)

	meta, repository := getAuthenticatedRepoAndMeta(ctx, rc, p, true)
	if meta == nil {
		// Status already written in getAuthenticatedRepoAndMeta
		return
	}

	contentStore := lfs_module.NewContentStore()
	defer ctx.Req.Body.Close()
	if err := contentStore.Put(meta.Pointer, ctx.Req.Body); err != nil {
		// Put will log the error itself
		ctx.Resp.WriteHeader(http.StatusInternalServerError)
		if err == lfs_module.ErrSizeMismatch || err == lfs_module.ErrHashMismatch {
			fmt.Fprintf(ctx.Resp, `{"message":"%s"}`, err)
		} else {
			fmt.Fprintf(ctx.Resp, `{"message":"Internal Server Error"}`)
		}
		if _, err = repository.RemoveLFSMetaObjectByOid(p.Oid); err != nil {
			log.Error("Whilst removing metaobject for LFS OID[%s] due to preceding error there was another Error: %v", p.Oid, err)
		}
		return
	}

	logRequest(ctx.Req, http.StatusOK)
}

// VerifyHandler verify oid and its size from the content store
func VerifyHandler(ctx *context.Context) {
	rc, p := unpack(ctx)

	meta, _ := getAuthenticatedRepoAndMeta(ctx, rc, p, true)
	if meta == nil {
		return
	}

	contentStore := lfs_module.NewContentStore()
	ok, err := contentStore.Verify(meta.Pointer)
	if err != nil {
		// Error will be logged in Verify
		ctx.Resp.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(ctx.Resp, `{"message":"Internal Server Error"}`)
		return
	}

	status := http.StatusOK
	if !ok {
		status = http.StatusUnprocessableEntity
	}
	writeStatus(ctx, status)
}

func getRequestContext(ctx *context.Context) *requestContext {
	return &requestContext{
		User:          ctx.Params("username"),
		Repo:          strings.TrimSuffix(ctx.Params("reponame"), ".git"),
		Authorization: ctx.Req.Header.Get("Authorization"),
	}
}

func buildObjectResponse(rc *requestContext, pointer lfs_module.Pointer, download, upload bool, errorCode int) *lfs_module.ObjectResponse {
	rep := &lfs_module.ObjectResponse{Pointer: pointer}
	if errorCode > 0 {
		rep.Error = &lfs_module.ObjectError{
			Code:    errorCode,
			Message: http.StatusText(errorCode),
		}
	} else {
		rep.Actions = make(map[string]*lfs_module.Link)

		header := make(map[string]string)
		verifyHeader := make(map[string]string)

		if len(rc.Authorization) > 0 {
			header["Authorization"] = rc.Authorization
			verifyHeader["Authorization"] = rc.Authorization
		}

		if download {
			rep.Actions["download"] = &lfs_module.Link{Href: rc.ObjectLink(pointer.Oid), Header: header}
		}
		if upload {
			rep.Actions["upload"] = &lfs_module.Link{Href: rc.ObjectLink(pointer.Oid), Header: header}
			rep.Actions["verify"] = &lfs_module.Link{Href: rc.VerifyLink(pointer.Oid), Header: verifyHeader}
		}
	}
	return rep
}

func unpack(ctx *context.Context) (*requestContext, lfs_module.Pointer) {
	rc := getRequestContext(ctx)
	p := lfs_module.Pointer{Oid: ctx.Params("oid")}

	if ctx.Req.Method == "POST" { // Maybe also check if +json
		var p2 lfs_module.Pointer
		bodyReader := ctx.Req.Body
		defer bodyReader.Close()
		json := jsoniter.ConfigCompatibleWithStandardLibrary
		dec := json.NewDecoder(bodyReader)
		err := dec.Decode(&p2)
		if err != nil {
			// The error is logged as a WARN here because this may represent misbehaviour rather than a true error
			log.Warn("Unable to decode POST request vars for LFS OID[%s] in %s/%s: Error: %v", p.Oid, rc.User, rc.Repo, err)
			return rc, p
		}

		p.Oid = p2.Oid
		p.Size = p2.Size
	}

	return rc, p
}

// TODO cheap hack, unify with unpack
func unpackbatch(ctx *context.Context) *lfs_module.BatchRequest {

	r := ctx.Req
	var bv lfs_module.BatchRequest

	bodyReader := r.Body
	defer bodyReader.Close()
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	dec := json.NewDecoder(bodyReader)
	err := dec.Decode(&bv)
	if err != nil {
		// The error is logged as a WARN here because this may represent misbehaviour rather than a true error
		log.Warn("Unable to decode BATCH request vars in %s/%s: Error: %v", ctx.Params("username"), strings.TrimSuffix(ctx.Params("reponame"), ".git"), err)
		return &bv
	}

	return &bv
}

func writeStatus(ctx *context.Context, status int) {
	message := http.StatusText(status)

	mediaParts := strings.Split(ctx.Req.Header.Get("Accept"), ";")
	mt := mediaParts[0]
	if strings.HasSuffix(mt, "+json") {
		message = `{"message":"` + message + `"}`
	}

	ctx.Resp.WriteHeader(status)
	fmt.Fprint(ctx.Resp, message)
	logRequest(ctx.Req, status)
}

func logRequest(r *http.Request, status int) {
	log.Debug("LFS request - Method: %s, URL: %s, Status %d", r.Method, r.URL, status)
}

// authenticate uses the authorization string to determine whether
// or not to proceed. This server assumes an HTTP Basic auth format.
func authenticate(ctx *context.Context, repository *models.Repository, authorization string, requireWrite bool) bool {
	accessMode := models.AccessModeRead
	if requireWrite {
		accessMode = models.AccessModeWrite
	}

	// ctx.IsSigned is unnecessary here, this will be checked in perm.CanAccess
	perm, err := models.GetUserRepoPermission(repository, ctx.User)
	if err != nil {
		log.Error("Unable to GetUserRepoPermission for user %-v in repo %-v Error: %v", ctx.User, repository)
		return false
	}

	canRead := perm.CanAccess(accessMode, models.UnitTypeCode)
	if canRead {
		return true
	}

	user, repo, opStr, err := parseToken(authorization)
	if err != nil {
		// Most of these are Warn level - the true internal server errors are logged in parseToken already
		log.Warn("Authentication failure for provided token with Error: %v", err)
		return false
	}
	ctx.User = user
	if opStr == "basic" {
		perm, err = models.GetUserRepoPermission(repository, ctx.User)
		if err != nil {
			log.Error("Unable to GetUserRepoPermission for user %-v in repo %-v Error: %v", ctx.User, repository)
			return false
		}
		return perm.CanAccess(accessMode, models.UnitTypeCode)
	}
	if repository.ID == repo.ID {
		if requireWrite && opStr != "upload" {
			return false
		}
		return true
	}
	return false
}

func parseToken(authorization string) (*models.User, *models.Repository, string, error) {
	if authorization == "" {
		return nil, nil, "unknown", fmt.Errorf("No token")
	}
	if strings.HasPrefix(authorization, "Bearer ") {
		token, err := jwt.ParseWithClaims(authorization[7:], &Claims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return setting.LFS.JWTSecretBytes, nil
		})
		if err != nil {
			// The error here is WARN level because it is caused by bad authorization rather than an internal server error
			return nil, nil, "unknown", err
		}
		claims, claimsOk := token.Claims.(*Claims)
		if !token.Valid || !claimsOk {
			return nil, nil, "unknown", fmt.Errorf("Token claim invalid")
		}
		r, err := models.GetRepositoryByID(claims.RepoID)
		if err != nil {
			log.Error("Unable to GetRepositoryById[%d]: Error: %v", claims.RepoID, err)
			return nil, nil, claims.Op, err
		}
		u, err := models.GetUserByID(claims.UserID)
		if err != nil {
			log.Error("Unable to GetUserById[%d]: Error: %v", claims.UserID, err)
			return nil, r, claims.Op, err
		}
		return u, r, claims.Op, nil
	}

	if strings.HasPrefix(authorization, "Basic ") {
		c, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authorization, "Basic "))
		if err != nil {
			return nil, nil, "basic", err
		}
		cs := string(c)
		i := strings.IndexByte(cs, ':')
		if i < 0 {
			return nil, nil, "basic", fmt.Errorf("Basic auth invalid")
		}
		user, password := cs[:i], cs[i+1:]
		u, err := models.GetUserByName(user)
		if err != nil {
			log.Error("Unable to GetUserByName[%d]: Error: %v", user, err)
			return nil, nil, "basic", err
		}
		if !u.IsPasswordSet() || !u.ValidatePassword(password) {
			return nil, nil, "basic", fmt.Errorf("Basic auth failed")
		}
		return u, nil, "basic", nil
	}

	return nil, nil, "unknown", fmt.Errorf("Token not found")
}

func requireAuth(ctx *context.Context) {
	ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
	writeStatus(ctx, http.StatusUnauthorized)
}
