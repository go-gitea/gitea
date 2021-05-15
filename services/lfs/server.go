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
func (rc *requestContext) VerifyLink() string {
	return setting.AppURL + path.Join(rc.User, rc.Repo+".git", "info/lfs/verify")
}

var oidRegExp = regexp.MustCompile(`^[A-Fa-f0-9]+$`)

func isOidValid(oid string) bool {
	return oidRegExp.MatchString(oid)
}

// ObjectOidHandler is the main request routing entry point into LFS server functions
func ObjectOidHandler(ctx *context.Context) {
	if !setting.LFS.StartServer {
		log.Debug("Attempt to access LFS server but LFS server is disabled")
		writeStatus(ctx, 404)
		return
	}

	if ctx.Req.Method == "GET" || ctx.Req.Method == "HEAD" {
		if MetaMatcher(ctx.Req) {
			getMetaHandler(ctx)
			return
		}

		getContentHandler(ctx)
		return
	} else if ctx.Req.Method == "PUT" {
		PutHandler(ctx)
		return
	}

	log.Warn("Unhandled LFS method: %s for %s/%s OID[%s]", ctx.Req.Method, ctx.Params("username"), ctx.Params("reponame"), ctx.Params("oid"))
	writeStatus(ctx, 404)
}

func getAuthenticatedRepoAndMeta(ctx *context.Context, rc *requestContext, p lfs_module.Pointer, requireWrite bool) (*models.LFSMetaObject, *models.Repository) {
	if !isOidValid(p.Oid) {
		log.Info("Attempt to access invalid LFS OID[%s] in %s/%s", p.Oid, rc.User, rc.Repo)
		writeStatus(ctx, 404)
		return nil, nil
	}

	repository, err := models.GetRepositoryByOwnerAndName(rc.User, rc.Repo)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", rc.User, rc.Repo, err)
		writeStatus(ctx, 404)
		return nil, nil
	}

	if !authenticate(ctx, repository, rc.Authorization, false, requireWrite) {
		requireAuth(ctx)
		return nil, nil
	}

	meta, err := repository.GetLFSMetaObjectByOid(p.Oid)
	if err != nil {
		log.Error("Unable to get LFS OID[%s] Error: %v", p.Oid, err)
		writeStatus(ctx, 404)
		return nil, nil
	}

	return meta, repository
}

// getContentHandler gets the content from the content store
func getContentHandler(ctx *context.Context) {
	rc, p := unpack(ctx)

	meta, _ := getAuthenticatedRepoAndMeta(ctx, rc, p, false)
	if meta == nil {
		// Status already written in getAuthenticatedRepoAndMeta
		return
	}

	// Support resume download using Range header
	var fromByte, toByte int64
	toByte = meta.Size - 1
	statusCode := 200
	if rangeHdr := ctx.Req.Header.Get("Range"); rangeHdr != "" {
		regex := regexp.MustCompile(`bytes=(\d+)\-(\d*).*`)
		match := regex.FindStringSubmatch(rangeHdr)
		if len(match) > 1 {
			statusCode = 206
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

// getMetaHandler retrieves metadata about the object
func getMetaHandler(ctx *context.Context) {
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
		if err := enc.Encode(represent(rc, meta.Pointer, true, false)); err != nil {
			log.Error("Failed to encode representation as json. Error: %v", err)
		}
	}

	logRequest(ctx.Req, 200)
}

// PostHandler instructs the client how to upload data
func PostHandler(ctx *context.Context) {
	if !setting.LFS.StartServer {
		log.Debug("Attempt to access LFS server but LFS server is disabled")
		writeStatus(ctx, 404)
		return
	}

	if !MetaMatcher(ctx.Req) {
		log.Info("Attempt to POST without accepting the correct media type: %s", lfs_module.MediaType)
		writeStatus(ctx, 400)
		return
	}

	rc, p := unpack(ctx)

	repository, err := models.GetRepositoryByOwnerAndName(rc.User, rc.Repo)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", rc.User, rc.Repo, err)
		writeStatus(ctx, 404)
		return
	}

	if !authenticate(ctx, repository, rc.Authorization, false, true) {
		requireAuth(ctx)
		return
	}

	if !isOidValid(p.Oid) {
		log.Info("Invalid LFS OID[%s] attempt to POST in %s/%s", p.Oid, rc.User, rc.Repo)
		writeStatus(ctx, 404)
		return
	}

	if setting.LFS.MaxFileSize > 0 && p.Size > setting.LFS.MaxFileSize {
		log.Info("Denied LFS OID[%s] upload of size %d to %s/%s because of LFS_MAX_FILE_SIZE=%d", p.Oid, p.Size, rc.User, rc.Repo, setting.LFS.MaxFileSize)
		writeStatus(ctx, 413)
		return
	}

	meta, err := models.NewLFSMetaObject(&models.LFSMetaObject{Pointer: p, RepositoryID: repository.ID})
	if err != nil {
		log.Error("Unable to write LFS OID[%s] size %d meta object in %v/%v to database. Error: %v", p.Oid, p.Size, rc.User, rc.Repo, err)
		writeStatus(ctx, 404)
		return
	}

	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	sentStatus := 202
	contentStore := lfs_module.NewContentStore()
	exist, err := contentStore.Exists(p)
	if err != nil {
		log.Error("Unable to check if LFS OID[%s] exist on %s / %s. Error: %v", p.Oid, rc.User, rc.Repo, err)
		writeStatus(ctx, 500)
		return
	}
	if meta.Existing && exist {
		sentStatus = 200
	}
	ctx.Resp.WriteHeader(sentStatus)

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	enc := json.NewEncoder(ctx.Resp)
	if err := enc.Encode(represent(rc, meta.Pointer, meta.Existing, true)); err != nil {
		log.Error("Failed to encode representation as json. Error: %v", err)
	}
	logRequest(ctx.Req, sentStatus)
}

// BatchHandler provides the batch api
func BatchHandler(ctx *context.Context) {
	if !setting.LFS.StartServer {
		log.Debug("Attempt to access LFS server but LFS server is disabled")
		writeStatus(ctx, 404)
		return
	}

	if !MetaMatcher(ctx.Req) {
		log.Info("Attempt to BATCH without accepting the correct media type: %s", lfs_module.MediaType)
		writeStatus(ctx, 400)
		return
	}

	bv := unpackbatch(ctx)

	reqCtx := &requestContext{
		User:          ctx.Params("username"),
		Repo:          strings.TrimSuffix(ctx.Params("reponame"), ".git"),
		Authorization: ctx.Req.Header.Get("Authorization"),
	}

	var responseObjects []*lfs_module.ObjectResponse

	// Create a response object
	for _, object := range bv.Objects {
		if !isOidValid(object.Oid) {
			log.Info("Invalid LFS OID[%s] attempt to BATCH in %s/%s", object.Oid, reqCtx.User, reqCtx.Repo)
			continue
		}

		repository, err := models.GetRepositoryByOwnerAndName(reqCtx.User, reqCtx.Repo)
		if err != nil {
			log.Error("Unable to get repository: %s/%s Error: %v", reqCtx.User, reqCtx.Repo, err)
			writeStatus(ctx, 404)
			return
		}

		requireWrite := false
		if bv.Operation == "upload" {
			requireWrite = true
		}

		if !authenticate(ctx, repository, reqCtx.Authorization, false, requireWrite) {
			requireAuth(ctx)
			return
		}

		contentStore := lfs_module.NewContentStore()

		meta, err := repository.GetLFSMetaObjectByOid(object.Oid)
		if err == nil { // Object is found and exists
			exist, err := contentStore.Exists(meta.Pointer)
			if err != nil {
				log.Error("Unable to check if LFS OID[%s] exist on %s / %s. Error: %v", object.Oid, reqCtx.User, reqCtx.Repo, err)
				writeStatus(ctx, 500)
				return
			}
			if exist {
				responseObjects = append(responseObjects, represent(reqCtx, meta.Pointer, true, false))
				continue
			}
		}

		if requireWrite && setting.LFS.MaxFileSize > 0 && object.Size > setting.LFS.MaxFileSize {
			log.Info("Denied LFS OID[%s] upload of size %d to %s/%s because of LFS_MAX_FILE_SIZE=%d", object.Oid, object.Size, reqCtx.User, reqCtx.Repo, setting.LFS.MaxFileSize)
			writeStatus(ctx, 413)
			return
		}

		// Object is not found
		meta, err = models.NewLFSMetaObject(&models.LFSMetaObject{Pointer: object, RepositoryID: repository.ID})
		if err == nil {
			exist, err := contentStore.Exists(meta.Pointer)
			if err != nil {
				log.Error("Unable to check if LFS OID[%s] exist on %s / %s. Error: %v", object.Oid, reqCtx.User, reqCtx.Repo, err)
				writeStatus(ctx, 500)
				return
			}
			responseObjects = append(responseObjects, represent(reqCtx, meta.Pointer, meta.Existing, !exist))
		} else {
			log.Error("Unable to write LFS OID[%s] size %d meta object in %v/%v to database. Error: %v", object.Oid, object.Size, reqCtx.User, reqCtx.Repo, err)
		}
	}

	ctx.Resp.Header().Set("Content-Type", lfs_module.MediaType)

	respobj := &lfs_module.BatchResponse{Objects: responseObjects}

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	enc := json.NewEncoder(ctx.Resp)
	if err := enc.Encode(respobj); err != nil {
		log.Error("Failed to encode representation as json. Error: %v", err)
	}
	logRequest(ctx.Req, 200)
}

// PutHandler receives data from the client and puts it into the content store
func PutHandler(ctx *context.Context) {
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
		ctx.Resp.WriteHeader(500)
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

	logRequest(ctx.Req, 200)
}

// VerifyHandler verify oid and its size from the content store
func VerifyHandler(ctx *context.Context) {
	if !setting.LFS.StartServer {
		log.Debug("Attempt to access LFS server but LFS server is disabled")
		writeStatus(ctx, 404)
		return
	}

	if !MetaMatcher(ctx.Req) {
		log.Info("Attempt to VERIFY without accepting the correct media type: %s", lfs_module.MediaType)
		writeStatus(ctx, 400)
		return
	}

	rc, p := unpack(ctx)

	meta, _ := getAuthenticatedRepoAndMeta(ctx, rc, p, true)
	if meta == nil {
		// Status already written in getAuthenticatedRepoAndMeta
		return
	}

	contentStore := lfs_module.NewContentStore()
	ok, err := contentStore.Verify(meta.Pointer)
	if err != nil {
		// Error will be logged in Verify
		ctx.Resp.WriteHeader(500)
		fmt.Fprintf(ctx.Resp, `{"message":"Internal Server Error"}`)
		return
	}
	if !ok {
		writeStatus(ctx, 422)
		return
	}

	logRequest(ctx.Req, 200)
}

// represent takes a requestContext and Meta and turns it into a ObjectResponse suitable
// for json encoding
func represent(rc *requestContext, pointer lfs_module.Pointer, download, upload bool) *lfs_module.ObjectResponse {
	rep := &lfs_module.ObjectResponse{
		Pointer: pointer,
		Actions: make(map[string]*lfs_module.Link),
	}

	header := make(map[string]string)

	if rc.Authorization == "" {
		//https://github.com/github/git-lfs/issues/1088
		header["Authorization"] = "Authorization: Basic dummy"
	} else {
		header["Authorization"] = rc.Authorization
	}

	if download {
		rep.Actions["download"] = &lfs_module.Link{Href: rc.ObjectLink(pointer.Oid), Header: header}
	}

	if upload {
		rep.Actions["upload"] = &lfs_module.Link{Href: rc.ObjectLink(pointer.Oid), Header: header}
	}

	if upload && !download {
		// Force client side verify action while gitea lacks proper server side verification
		verifyHeader := make(map[string]string)
		for k, v := range header {
			verifyHeader[k] = v
		}

		// This is only needed to workaround https://github.com/git-lfs/git-lfs/issues/3662
		verifyHeader["Accept"] = lfs_module.MediaType

		rep.Actions["verify"] = &lfs_module.Link{Href: rc.VerifyLink(), Header: verifyHeader}
	}

	return rep
}

// MetaMatcher provides a mux.MatcherFunc that only allows requests that contain
// an Accept header with the lfs_module.MediaType
func MetaMatcher(r *http.Request) bool {
	mediaParts := strings.Split(r.Header.Get("Accept"), ";")
	mt := mediaParts[0]
	return mt == lfs_module.MediaType
}

func unpack(ctx *context.Context) (*requestContext, lfs_module.Pointer) {
	r := ctx.Req
	rc := &requestContext{
		User:          ctx.Params("username"),
		Repo:          strings.TrimSuffix(ctx.Params("reponame"), ".git"),
		Authorization: r.Header.Get("Authorization"),
	}
	p := lfs_module.Pointer{Oid: ctx.Params("oid")}

	if r.Method == "POST" { // Maybe also check if +json
		var p2 lfs_module.Pointer
		bodyReader := r.Body
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
func authenticate(ctx *context.Context, repository *models.Repository, authorization string, requireSigned, requireWrite bool) bool {
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
	if canRead && (!requireSigned || ctx.IsSigned) {
		return true
	}

	user, err := parseToken(authorization, repository, accessMode)
	if err != nil {
		// Most of these are Warn level - the true internal server errors are logged in parseToken already
		log.Warn("Authentication failure for provided token with Error: %v", err)
		return false
	}
	ctx.User = user
	return true
}

func handleLFSToken(tokenSHA string, target *models.Repository, mode models.AccessMode) (*models.User, error) {
	if !strings.Contains(tokenSHA, ".") {
		return nil, nil
	}
	token, err := jwt.ParseWithClaims(tokenSHA, &Claims{}, func(t *jwt.Token) (interface{}, error) {
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

	if mode == models.AccessModeWrite && claims.Op != "upload" {
		return nil, fmt.Errorf("invalid token claim")
	}

	u, err := models.GetUserByID(claims.UserID)
	if err != nil {
		log.Error("Unable to GetUserById[%d]: Error: %v", claims.UserID, err)
		return nil, err
	}
	return u, nil
}

func parseToken(authorization string, target *models.Repository, mode models.AccessMode) (*models.User, error) {
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
		return handleLFSToken(tokenSHA, target, mode)
	}
	return nil, fmt.Errorf("token not found")
}

func requireAuth(ctx *context.Context) {
	ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
	writeStatus(ctx, 401)
}
