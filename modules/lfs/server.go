package lfs

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"gitea.com/macaron/macaron"
	"github.com/dgrijalva/jwt-go"
)

const (
	metaMediaType = "application/vnd.git-lfs+json"
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
	return setting.AppURL + path.Join(v.User, v.Repo+".git", "info/lfs/objects", v.Oid)
}

// VerifyLink builds a URL for verifying the object.
func (v *RequestVars) VerifyLink() string {
	return setting.AppURL + path.Join(v.User, v.Repo+".git", "info/lfs/verify")
}

// link provides a structure used to build a hypermedia representation of an HTTP link.
type link struct {
	Href      string            `json:"href"`
	Header    map[string]string `json:"header,omitempty"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
}

var oidRegExp = regexp.MustCompile(`^[A-Fa-f0-9]+$`)

func isOidValid(oid string) bool {
	return oidRegExp.MatchString(oid)
}

// ObjectOidHandler is the main request routing entry point into LFS server functions
func ObjectOidHandler(ctx *context.Context) {
	if !setting.LFS.StartServer {
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

}

func getAuthenticatedRepoAndMeta(ctx *context.Context, rv *RequestVars, requireWrite bool) (*models.LFSMetaObject, *models.Repository) {
	if !isOidValid(rv.Oid) {
		writeStatus(ctx, 404)
		return nil, nil
	}

	repository, err := models.GetRepositoryByOwnerAndName(rv.User, rv.Repo)
	if err != nil {
		log.Debug("Could not find repository: %s/%s - %s", rv.User, rv.Repo, err)
		writeStatus(ctx, 404)
		return nil, nil
	}

	if !authenticate(ctx, repository, rv.Authorization, requireWrite) {
		requireAuth(ctx)
		return nil, nil
	}

	meta, err := repository.GetLFSMetaObjectByOid(rv.Oid)
	if err != nil {
		writeStatus(ctx, 404)
		return nil, nil
	}

	return meta, repository
}

// getContentHandler gets the content from the content store
func getContentHandler(ctx *context.Context) {
	rv := unpack(ctx)

	meta, _ := getAuthenticatedRepoAndMeta(ctx, rv, false)
	if meta == nil {
		return
	}

	// Support resume download using Range header
	var fromByte int64
	statusCode := 200
	if rangeHdr := ctx.Req.Header.Get("Range"); rangeHdr != "" {
		regex := regexp.MustCompile(`bytes=(\d+)\-.*`)
		match := regex.FindStringSubmatch(rangeHdr)
		if len(match) > 1 {
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

	ctx.Resp.Header().Set("Content-Length", strconv.FormatInt(meta.Size-fromByte, 10))
	ctx.Resp.Header().Set("Content-Type", "application/octet-stream")

	filename := ctx.Params("filename")
	if len(filename) > 0 {
		decodedFilename, err := base64.RawURLEncoding.DecodeString(filename)
		if err == nil {
			ctx.Resp.Header().Set("Content-Disposition", "attachment; filename=\""+string(decodedFilename)+"\"")
		}
	}

	ctx.Resp.WriteHeader(statusCode)
	_, _ = io.Copy(ctx.Resp, content)
	_ = content.Close()
	logRequest(ctx.Req, statusCode)
}

// getMetaHandler retrieves metadata about the object
func getMetaHandler(ctx *context.Context) {
	rv := unpack(ctx)

	meta, _ := getAuthenticatedRepoAndMeta(ctx, rv, false)
	if meta == nil {
		return
	}

	ctx.Resp.Header().Set("Content-Type", metaMediaType)

	if ctx.Req.Method == "GET" {
		enc := json.NewEncoder(ctx.Resp)
		_ = enc.Encode(Represent(rv, meta, true, false))
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

	repository, err := models.GetRepositoryByOwnerAndName(rv.User, rv.Repo)
	if err != nil {
		log.Debug("Could not find repository: %s/%s - %s", rv.User, rv.Repo, err)
		writeStatus(ctx, 404)
		return
	}

	if !authenticate(ctx, repository, rv.Authorization, true) {
		requireAuth(ctx)
		return
	}

	if !isOidValid(rv.Oid) {
		writeStatus(ctx, 404)
		return
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
	_ = enc.Encode(Represent(rv, meta, meta.Existing, true))
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
		if !isOidValid(object.Oid) {
			continue
		}

		repository, err := models.GetRepositoryByOwnerAndName(object.User, object.Repo)

		if err != nil {
			log.Debug("Could not find repository: %s/%s - %s", object.User, object.Repo, err)
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

		contentStore := &ContentStore{BasePath: setting.LFS.ContentPath}

		meta, err := repository.GetLFSMetaObjectByOid(object.Oid)
		if err == nil && contentStore.Exists(meta) { // Object is found and exists
			responseObjects = append(responseObjects, Represent(object, meta, true, false))
			continue
		}

		// Object is not found
		meta, err = models.NewLFSMetaObject(&models.LFSMetaObject{Oid: object.Oid, Size: object.Size, RepositoryID: repository.ID})
		if err == nil {
			responseObjects = append(responseObjects, Represent(object, meta, meta.Existing, !contentStore.Exists(meta)))
		}
	}

	ctx.Resp.Header().Set("Content-Type", metaMediaType)

	respobj := &BatchResponse{Objects: responseObjects}

	enc := json.NewEncoder(ctx.Resp)
	_ = enc.Encode(respobj)
	logRequest(ctx.Req, 200)
}

// PutHandler receives data from the client and puts it into the content store
func PutHandler(ctx *context.Context) {
	rv := unpack(ctx)

	meta, repository := getAuthenticatedRepoAndMeta(ctx, rv, true)
	if meta == nil {
		return
	}

	contentStore := &ContentStore{BasePath: setting.LFS.ContentPath}
	bodyReader := ctx.Req.Body().ReadCloser()
	defer bodyReader.Close()
	if err := contentStore.Put(meta, bodyReader); err != nil {
		ctx.Resp.WriteHeader(500)
		fmt.Fprintf(ctx.Resp, `{"message":"%s"}`, err)
		if _, err = repository.RemoveLFSMetaObjectByOid(rv.Oid); err != nil {
			log.Error("RemoveLFSMetaObjectByOid: %v", err)
		}
		return
	}

	logRequest(ctx.Req, 200)
}

// VerifyHandler verify oid and its size from the content store
func VerifyHandler(ctx *context.Context) {
	if !setting.LFS.StartServer {
		writeStatus(ctx, 404)
		return
	}

	if !MetaMatcher(ctx.Req) {
		writeStatus(ctx, 400)
		return
	}

	rv := unpack(ctx)

	meta, _ := getAuthenticatedRepoAndMeta(ctx, rv, true)
	if meta == nil {
		return
	}

	contentStore := &ContentStore{BasePath: setting.LFS.ContentPath}
	ok, err := contentStore.Verify(meta)
	if err != nil {
		ctx.Resp.WriteHeader(500)
		fmt.Fprintf(ctx.Resp, `{"message":"%s"}`, err)
		return
	}
	if !ok {
		writeStatus(ctx, 422)
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

	if upload && !download {
		// Force client side verify action while gitea lacks proper server side verification
		verifyHeader := make(map[string]string)
		for k, v := range header {
			verifyHeader[k] = v
		}

		// This is only needed to workaround https://github.com/git-lfs/git-lfs/issues/3662
		verifyHeader["Accept"] = metaMediaType

		rep.Actions["verify"] = &link{Href: rv.VerifyLink(), Header: verifyHeader}
	}

	return rep
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
		bodyReader := r.Body().ReadCloser()
		defer bodyReader.Close()
		dec := json.NewDecoder(bodyReader)
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

	bodyReader := r.Body().ReadCloser()
	defer bodyReader.Close()
	dec := json.NewDecoder(bodyReader)
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

	// ctx.IsSigned is unnecessary here, this will be checked in perm.CanAccess
	perm, err := models.GetUserRepoPermission(repository, ctx.User)
	if err != nil {
		return false
	}

	canRead := perm.CanAccess(accessMode, models.UnitTypeCode)
	if canRead {
		return true
	}

	user, repo, opStr, err := parseToken(authorization)
	if err != nil {
		return false
	}
	ctx.User = user
	if opStr == "basic" {
		perm, err = models.GetUserRepoPermission(repository, ctx.User)
		if err != nil {
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
		token, err := jwt.Parse(authorization[7:], func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return setting.LFS.JWTSecretBytes, nil
		})
		if err != nil {
			return nil, nil, "unknown", err
		}
		claims, claimsOk := token.Claims.(jwt.MapClaims)
		if !token.Valid || !claimsOk {
			return nil, nil, "unknown", fmt.Errorf("Token claim invalid")
		}
		opStr, ok := claims["op"].(string)
		if !ok {
			return nil, nil, "unknown", fmt.Errorf("Token operation invalid")
		}
		repoID, ok := claims["repo"].(float64)
		if !ok {
			return nil, nil, opStr, fmt.Errorf("Token repository id invalid")
		}
		r, err := models.GetRepositoryByID(int64(repoID))
		if err != nil {
			return nil, nil, opStr, err
		}
		userID, ok := claims["user"].(float64)
		if !ok {
			return nil, r, opStr, fmt.Errorf("Token user id invalid")
		}
		u, err := models.GetUserByID(int64(userID))
		if err != nil {
			return nil, r, opStr, err
		}
		return u, r, opStr, nil
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
	writeStatus(ctx, 401)
}
