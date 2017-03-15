package lfs

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"github.com/dgrijalva/jwt-go"
	"gopkg.in/macaron.v1"
)

const (
	contentMediaType = "application/vnd.git-lfs"
	metaMediaType    = contentMediaType + "+json"
)

// RequestVars contain variables from the HTTP request. Variables from routing, json body decoding, and
// some headers are stored.
type RequestVars struct {
	Oid           string
	Size          int64
	User          string
	Password      string
	Repo          string
	Authorization string
}

// BatchVars contains multiple RequestVars processed in one batch operation.
// https://github.com/git-lfs/git-lfs/blob/master/docs/api/batch.md
type BatchVars struct {
	Transfers []string       `json:"transfers,omitempty"`
	Operation string         `json:"operation"`
	Objects   []*RequestVars `json:"objects"`
}

// BatchResponse contains multiple object metadata Representation structures
// for use with the batch API.
type BatchResponse struct {
	Transfer string            `json:"transfer,omitempty"`
	Objects  []*Representation `json:"objects"`
}

// Representation is object metadata as seen by clients of the lfs server.
type Representation struct {
	Oid     string           `json:"oid"`
	Size    int64            `json:"size"`
	Actions map[string]*link `json:"actions"`
	Error   *ObjectError     `json:"error,omitempty"`
}

// ObjectError defines the JSON structure returned to the client in case of an error
type ObjectError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ObjectLink builds a URL linking to the object.
func (v *RequestVars) ObjectLink() string {
	return fmt.Sprintf("%s%s/%s/info/lfs/objects/%s", setting.AppURL, v.User, v.Repo, v.Oid)
}

// link provides a structure used to build a hypermedia representation of an HTTP link.
type link struct {
	Href      string            `json:"href"`
	Header    map[string]string `json:"header,omitempty"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
}

// ObjectOidHandler is the main request routing entry point into LFS server functions
func ObjectOidHandler(ctx *context.Context) {

	if !setting.LFS.StartServer {
		writeStatus(ctx, 404)
		return
	}

	if ctx.Req.Method == "GET" || ctx.Req.Method == "HEAD" {
		if MetaMatcher(ctx.Req) {
			GetMetaHandler(ctx)
			return
		}
		if ContentMatcher(ctx.Req) || len(ctx.Params("filename")) > 0 {
			GetContentHandler(ctx)
			return
		}
	} else if ctx.Req.Method == "PUT" && ContentMatcher(ctx.Req) {
		PutHandler(ctx)
		return
	}

}

// GetContentHandler gets the content from the content store
func GetContentHandler(ctx *context.Context) {

	rv := unpack(ctx)

	meta, err := models.GetLFSMetaObjectByOid(rv.Oid)
	if err != nil {
		writeStatus(ctx, 404)
		return
	}

	repository, err := models.GetRepositoryByID(meta.RepositoryID)

	if err != nil {
		writeStatus(ctx, 404)
		return
	}

	if !authenticate(ctx, repository, rv.Authorization, false) {
		requireAuth(ctx)
		return
	}

	// Support resume download using Range header
	var fromByte int64
	statusCode := 200
	if rangeHdr := ctx.Req.Header.Get("Range"); rangeHdr != "" {
		regex := regexp.MustCompile(`bytes=(\d+)\-.*`)
		match := regex.FindStringSubmatch(rangeHdr)
		if match != nil && len(match) > 1 {
			statusCode = 206
			fromByte, _ = strconv.ParseInt(match[1], 10, 32)
			ctx.Resp.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", fromByte, meta.Size-1, meta.Size-fromByte))
		}
	}

	contentStore := &ContentStore{BasePath: setting.LFS.ContentPath}
	content, err := contentStore.Get(meta, fromByte)
	if err != nil {
		writeStatus(ctx, 404)
		return
	}

	ctx.Resp.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
	ctx.Resp.Header().Set("Content-Type", "application/octet-stream")

	filename := ctx.Params("filename")
	if len(filename) > 0 {
		decodedFilename, err := base64.RawURLEncoding.DecodeString(filename)
		if err == nil {
			ctx.Resp.Header().Set("Content-Disposition", "attachment; filename=\""+string(decodedFilename)+"\"")
		}
	}

	ctx.Resp.WriteHeader(statusCode)
	io.Copy(ctx.Resp, content)
	content.Close()
	logRequest(ctx.Req, statusCode)
}

// GetMetaHandler retrieves metadata about the object
func GetMetaHandler(ctx *context.Context) {

	rv := unpack(ctx)

	meta, err := models.GetLFSMetaObjectByOid(rv.Oid)
	if err != nil {
		writeStatus(ctx, 404)
		return
	}

	repository, err := models.GetRepositoryByID(meta.RepositoryID)

	if err != nil {
		writeStatus(ctx, 404)
		return
	}

	if !authenticate(ctx, repository, rv.Authorization, false) {
		requireAuth(ctx)
		return
	}

	ctx.Resp.Header().Set("Content-Type", metaMediaType)

	if ctx.Req.Method == "GET" {
		enc := json.NewEncoder(ctx.Resp)
		enc.Encode(Represent(rv, meta, true, false))
	}

	logRequest(ctx.Req, 200)
}

// PostHandler instructs the client how to upload data
func PostHandler(ctx *context.Context) {

	if !setting.LFS.StartServer {
		writeStatus(ctx, 404)
		return
	}

	if !MetaMatcher(ctx.Req) {
		writeStatus(ctx, 400)
		return
	}

	rv := unpack(ctx)

	repositoryString := rv.User + "/" + rv.Repo
	repository, err := models.GetRepositoryByRef(repositoryString)

	if err != nil {
		log.Debug("Could not find repository: %s - %s", repositoryString, err)
		writeStatus(ctx, 404)
		return
	}

	if !authenticate(ctx, repository, rv.Authorization, true) {
		requireAuth(ctx)
	}

	meta, err := models.NewLFSMetaObject(&models.LFSMetaObject{Oid: rv.Oid, Size: rv.Size, RepositoryID: repository.ID})

	if err != nil {
		writeStatus(ctx, 404)
		return
	}

	ctx.Resp.Header().Set("Content-Type", metaMediaType)

	sentStatus := 202
	contentStore := &ContentStore{BasePath: setting.LFS.ContentPath}
	if meta.Existing && contentStore.Exists(meta) {
		sentStatus = 200
	}
	ctx.Resp.WriteHeader(sentStatus)

	enc := json.NewEncoder(ctx.Resp)
	enc.Encode(Represent(rv, meta, meta.Existing, true))
	logRequest(ctx.Req, sentStatus)
}

// BatchHandler provides the batch api
func BatchHandler(ctx *context.Context) {

	if !setting.LFS.StartServer {
		writeStatus(ctx, 404)
		return
	}

	if !MetaMatcher(ctx.Req) {
		writeStatus(ctx, 400)
		return
	}

	bv := unpackbatch(ctx)

	var responseObjects []*Representation

	// Create a response object
	for _, object := range bv.Objects {

		repositoryString := object.User + "/" + object.Repo
		repository, err := models.GetRepositoryByRef(repositoryString)

		if err != nil {
			log.Debug("Could not find repository: %s - %s", repositoryString, err)
			writeStatus(ctx, 404)
			return
		}

		requireWrite := false
		if bv.Operation == "upload" {
			requireWrite = true
		}

		if !authenticate(ctx, repository, object.Authorization, requireWrite) {
			requireAuth(ctx)
			return
		}

		meta, err := models.GetLFSMetaObjectByOid(object.Oid)

		contentStore := &ContentStore{BasePath: setting.LFS.ContentPath}
		if err == nil && contentStore.Exists(meta) { // Object is found and exists
			responseObjects = append(responseObjects, Represent(object, meta, true, false))
			continue
		}

		// Object is not found
		meta, err = models.NewLFSMetaObject(&models.LFSMetaObject{Oid: object.Oid, Size: object.Size, RepositoryID: repository.ID})

		if err == nil {
			responseObjects = append(responseObjects, Represent(object, meta, meta.Existing, true))
		}
	}

	ctx.Resp.Header().Set("Content-Type", metaMediaType)

	respobj := &BatchResponse{Objects: responseObjects}

	enc := json.NewEncoder(ctx.Resp)
	enc.Encode(respobj)
	logRequest(ctx.Req, 200)
}

// PutHandler receives data from the client and puts it into the content store
func PutHandler(ctx *context.Context) {
	rv := unpack(ctx)

	meta, err := models.GetLFSMetaObjectByOid(rv.Oid)

	if err != nil {
		writeStatus(ctx, 404)
		return
	}

	repository, err := models.GetRepositoryByID(meta.RepositoryID)

	if err != nil {
		writeStatus(ctx, 404)
		return
	}

	if !authenticate(ctx, repository, rv.Authorization, true) {
		requireAuth(ctx)
		return
	}

	contentStore := &ContentStore{BasePath: setting.LFS.ContentPath}
	if err := contentStore.Put(meta, ctx.Req.Body().ReadCloser()); err != nil {
		models.RemoveLFSMetaObjectByOid(rv.Oid)
		ctx.Resp.WriteHeader(500)
		fmt.Fprintf(ctx.Resp, `{"message":"%s"}`, err)
		return
	}

	logRequest(ctx.Req, 200)
}

// Represent takes a RequestVars and Meta and turns it into a Representation suitable
// for json encoding
func Represent(rv *RequestVars, meta *models.LFSMetaObject, download, upload bool) *Representation {
	rep := &Representation{
		Oid:     meta.Oid,
		Size:    meta.Size,
		Actions: make(map[string]*link),
	}

	header := make(map[string]string)
	header["Accept"] = contentMediaType

	if rv.Authorization == "" {
		//https://github.com/github/git-lfs/issues/1088
		header["Authorization"] = "Authorization: Basic dummy"
	} else {
		header["Authorization"] = rv.Authorization
	}

	if download {
		rep.Actions["download"] = &link{Href: rv.ObjectLink(), Header: header}
	}

	if upload {
		rep.Actions["upload"] = &link{Href: rv.ObjectLink(), Header: header}
	}

	return rep
}

// ContentMatcher provides a mux.MatcherFunc that only allows requests that contain
// an Accept header with the contentMediaType
func ContentMatcher(r macaron.Request) bool {
	mediaParts := strings.Split(r.Header.Get("Accept"), ";")
	mt := mediaParts[0]
	return mt == contentMediaType
}

// MetaMatcher provides a mux.MatcherFunc that only allows requests that contain
// an Accept header with the metaMediaType
func MetaMatcher(r macaron.Request) bool {
	mediaParts := strings.Split(r.Header.Get("Accept"), ";")
	mt := mediaParts[0]
	return mt == metaMediaType
}

func unpack(ctx *context.Context) *RequestVars {
	r := ctx.Req
	rv := &RequestVars{
		User:          ctx.Params("username"),
		Repo:          strings.TrimSuffix(ctx.Params("reponame"), ".git"),
		Oid:           ctx.Params("oid"),
		Authorization: r.Header.Get("Authorization"),
	}

	if r.Method == "POST" { // Maybe also check if +json
		var p RequestVars
		dec := json.NewDecoder(r.Body().ReadCloser())
		err := dec.Decode(&p)
		if err != nil {
			return rv
		}

		rv.Oid = p.Oid
		rv.Size = p.Size
	}

	return rv
}

// TODO cheap hack, unify with unpack
func unpackbatch(ctx *context.Context) *BatchVars {

	r := ctx.Req
	var bv BatchVars

	dec := json.NewDecoder(r.Body().ReadCloser())
	err := dec.Decode(&bv)
	if err != nil {
		return &bv
	}

	for i := 0; i < len(bv.Objects); i++ {
		bv.Objects[i].User = ctx.Params("username")
		bv.Objects[i].Repo = strings.TrimSuffix(ctx.Params("reponame"), ".git")
		bv.Objects[i].Authorization = r.Header.Get("Authorization")
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

func logRequest(r macaron.Request, status int) {
	log.Debug("LFS request - Method: %s, URL: %s, Status %d", r.Method, r.URL, status)
}

// authenticate uses the authorization string to determine whether
// or not to proceed. This server assumes an HTTP Basic auth format.
func authenticate(ctx *context.Context, repository *models.Repository, authorization string, requireWrite bool) bool {

	accessMode := models.AccessModeRead
	if requireWrite {
		accessMode = models.AccessModeWrite
	}

	if !repository.IsPrivate && !requireWrite {
		return true
	}

	if ctx.IsSigned {
		accessCheck, _ := models.HasAccess(ctx.User.ID, repository, accessMode)
		return accessCheck
	}

	if authorization == "" {
		return false
	}

	if authenticateToken(repository, authorization, requireWrite) {
		return true
	}

	if !strings.HasPrefix(authorization, "Basic ") {
		return false
	}

	c, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authorization, "Basic "))
	if err != nil {
		return false
	}
	cs := string(c)
	i := strings.IndexByte(cs, ':')
	if i < 0 {
		return false
	}
	user, password := cs[:i], cs[i+1:]

	userModel, err := models.GetUserByName(user)
	if err != nil {
		return false
	}

	if !userModel.ValidatePassword(password) {
		return false
	}

	accessCheck, _ := models.HasAccess(userModel.ID, repository, accessMode)
	return accessCheck
}

func authenticateToken(repository *models.Repository, authorization string, requireWrite bool) bool {
	if !strings.HasPrefix(authorization, "Bearer ") {
		return false
	}

	token, err := jwt.Parse(authorization[7:], func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return setting.LFS.JWTSecretBytes, nil
	})
	if err != nil {
		return false
	}
	claims, claimsOk := token.Claims.(jwt.MapClaims)
	if !token.Valid || !claimsOk {
		return false
	}

	opStr, ok := claims["op"].(string)
	if !ok {
		return false
	}

	if requireWrite && opStr != "upload" {
		return false
	}

	repoID, ok := claims["repo"].(float64)
	if !ok {
		return false
	}

	if repository.ID != int64(repoID) {
		return false
	}

	return true
}

func requireAuth(ctx *context.Context) {
	ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=gitea-lfs")
	writeStatus(ctx, 401)
}
