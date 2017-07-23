// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/com"
)

func composeGoGetImport(owner, repo, sub string) string {
	return path.Join(setting.Domain, setting.AppSubURL, owner, repo, sub)
}

// earlyResponseForGoGetMeta responses appropriate go-get meta with status 200
// if user does not have actual access to the requested repository,
// or the owner or repository does not exist at all.
// This is particular a workaround for "go get" command which does not respect
// .netrc file.
func earlyResponseForGoGetMeta(ctx *context.Context, username, reponame, subpath string) {
	ctx.PlainText(200, []byte(com.Expand(`<meta name="go-import" content="{GoGetImport} git {CloneLink}">`,
		map[string]string{
			"GoGetImport": composeGoGetImport(username, reponame, subpath),
			"CloneLink":   models.ComposeHTTPSCloneURL(username, reponame),
		})))
}

// HTTP implmentation git smart HTTP protocol
func HTTP(ctx *context.Context) {
	username := ctx.Params(":username")
	reponame := strings.TrimSuffix(ctx.Params(":reponame"), ".git")
	subpath := ctx.Params("*")

	if ctx.Query("go-get") == "1" {
		earlyResponseForGoGetMeta(ctx, username, reponame, subpath)
		return
	}

	var isPull bool
	service := ctx.Query("service")
	if service == "git-receive-pack" ||
		strings.HasSuffix(ctx.Req.URL.Path, "git-receive-pack") {
		isPull = false
	} else if service == "git-upload-pack" ||
		strings.HasSuffix(ctx.Req.URL.Path, "git-upload-pack") {
		isPull = true
	} else if service == "git-upload-archive" ||
		strings.HasSuffix(ctx.Req.URL.Path, "git-upload-archive") {
		isPull = true
	} else {
		isPull = (ctx.Req.Method == "GET")
	}

	var accessMode models.AccessMode
	if isPull {
		accessMode = models.AccessModeRead
	} else {
		accessMode = models.AccessModeWrite
	}

	isWiki := false
	var unitType = models.UnitTypeCode
	if strings.HasSuffix(reponame, ".wiki") {
		isWiki = true
		unitType = models.UnitTypeWiki
		reponame = reponame[:len(reponame)-5]
	}

	repoUser, err := models.GetUserByName(username)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Handle(http.StatusNotFound, "GetUserByName", nil)
		} else {
			ctx.Handle(http.StatusInternalServerError, "GetUserByName", err)
		}
		return
	}

	repo, err := models.GetRepositoryByName(repoUser.ID, reponame)
	if err != nil {
		if models.IsErrRepoNotExist(err) {
			ctx.Handle(http.StatusNotFound, "GetRepositoryByName", nil)
		} else {
			ctx.Handle(http.StatusInternalServerError, "GetRepositoryByName", err)
		}
		return
	}

	// Only public pull don't need auth.
	isPublicPull := !repo.IsPrivate && isPull
	var (
		askAuth      = !isPublicPull || setting.Service.RequireSignInView
		authUser     *models.User
		authUsername string
		authPasswd   string
		environ      []string
	)

	// check access
	if askAuth {
		if setting.Service.EnableReverseProxyAuth {
			authUsername = ctx.Req.Header.Get(setting.ReverseProxyAuthUser)
			if len(authUsername) == 0 {
				ctx.HandleText(401, "reverse proxy login error. authUsername empty")
				return
			}
			authUser, err = models.GetUserByName(authUsername)
			if err != nil {
				ctx.HandleText(401, "reverse proxy login error, got error while running GetUserByName")
				return
			}
		} else {
			authHead := ctx.Req.Header.Get("Authorization")
			if len(authHead) == 0 {
				ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=\".\"")
				ctx.Error(http.StatusUnauthorized)
				return
			}

			auths := strings.Fields(authHead)
			// currently check basic auth
			// TODO: support digit auth
			// FIXME: middlewares/context.go did basic auth check already,
			// maybe could use that one.
			if len(auths) != 2 || auths[0] != "Basic" {
				ctx.HandleText(http.StatusUnauthorized, "no basic auth and digit auth")
				return
			}
			authUsername, authPasswd, err = base.BasicAuthDecode(auths[1])
			if err != nil {
				ctx.HandleText(http.StatusUnauthorized, "no basic auth and digit auth")
				return
			}

			authUser, err = models.UserSignIn(authUsername, authPasswd)
			if err != nil {
				if !models.IsErrUserNotExist(err) {
					ctx.Handle(http.StatusInternalServerError, "UserSignIn error: %v", err)
					return
				}
			}

			if authUser == nil {
				authUser, err = models.GetUserByName(authUsername)

				if err != nil {
					if models.IsErrUserNotExist(err) {
						ctx.HandleText(http.StatusUnauthorized, "invalid credentials")
					} else {
						ctx.Handle(http.StatusInternalServerError, "GetUserByName", err)
					}
					return
				}

				// Assume password is a token.
				token, err := models.GetAccessTokenBySHA(authPasswd)
				if err != nil {
					if models.IsErrAccessTokenNotExist(err) || models.IsErrAccessTokenEmpty(err) {
						ctx.HandleText(http.StatusUnauthorized, "invalid credentials")
					} else {
						ctx.Handle(http.StatusInternalServerError, "GetAccessTokenBySha", err)
					}
					return
				}

				if authUser.ID != token.UID {
					ctx.HandleText(http.StatusUnauthorized, "invalid credentials")
					return
				}

				token.Updated = time.Now()
				if err = models.UpdateAccessToken(token); err != nil {
					ctx.Handle(http.StatusInternalServerError, "UpdateAccessToken", err)
				}

			} else {
				_, err = models.GetTwoFactorByUID(authUser.ID)

				if err == nil {
					// TODO: This response should be changed to "invalid credentials" for security reasons once the expectation behind it (creating an app token to authenticate) is properly documented
					ctx.HandleText(http.StatusUnauthorized, "Users with two-factor authentication enabled cannot perform HTTP/HTTPS operations via plain username and password. Please create and use a personal access token on the user settings page")
					return
				} else if !models.IsErrTwoFactorNotEnrolled(err) {
					ctx.Handle(http.StatusInternalServerError, "IsErrTwoFactorNotEnrolled", err)
					return
				}
			}

			if !isPublicPull {
				has, err := models.HasAccess(authUser.ID, repo, accessMode)
				if err != nil {
					ctx.Handle(http.StatusInternalServerError, "HasAccess", err)
					return
				} else if !has {
					if accessMode == models.AccessModeRead {
						has, err = models.HasAccess(authUser.ID, repo, models.AccessModeWrite)
						if err != nil {
							ctx.Handle(http.StatusInternalServerError, "HasAccess2", err)
							return
						} else if !has {
							ctx.HandleText(http.StatusForbidden, "User permission denied")
							return
						}
					} else {
						ctx.HandleText(http.StatusForbidden, "User permission denied")
						return
					}
				}

				if !isPull && repo.IsMirror {
					ctx.HandleText(http.StatusForbidden, "mirror repository is read-only")
					return
				}
			}
		}

		if !repo.CheckUnitUser(authUser.ID, authUser.IsAdmin, unitType) {
			ctx.HandleText(http.StatusForbidden, fmt.Sprintf("User %s does not have allowed access to repository %s 's code",
				authUser.Name, repo.RepoPath()))
			return
		}

		environ = []string{
			models.EnvRepoUsername + "=" + username,
			models.EnvRepoName + "=" + reponame,
			models.EnvPusherName + "=" + authUser.Name,
			models.EnvPusherID + fmt.Sprintf("=%d", authUser.ID),
			models.ProtectedBranchRepoID + fmt.Sprintf("=%d", repo.ID),
		}
		if isWiki {
			environ = append(environ, models.EnvRepoIsWiki+"=true")
		} else {
			environ = append(environ, models.EnvRepoIsWiki+"=false")
		}
	}

	HTTPBackend(ctx, &serviceConfig{
		UploadPack:  true,
		ReceivePack: true,
		Env:         environ,
	})(ctx.Resp, ctx.Req.Request)
}

type serviceConfig struct {
	UploadPack  bool
	ReceivePack bool
	Env         []string
}

type serviceHandler struct {
	cfg     *serviceConfig
	w       http.ResponseWriter
	r       *http.Request
	dir     string
	file    string
	environ []string
}

func (h *serviceHandler) setHeaderNoCache() {
	h.w.Header().Set("Expires", "Fri, 01 Jan 1980 00:00:00 GMT")
	h.w.Header().Set("Pragma", "no-cache")
	h.w.Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
}

func (h *serviceHandler) setHeaderCacheForever() {
	now := time.Now().Unix()
	expires := now + 31536000
	h.w.Header().Set("Date", fmt.Sprintf("%d", now))
	h.w.Header().Set("Expires", fmt.Sprintf("%d", expires))
	h.w.Header().Set("Cache-Control", "public, max-age=31536000")
}

func (h *serviceHandler) sendFile(contentType string) {
	reqFile := path.Join(h.dir, h.file)

	fi, err := os.Stat(reqFile)
	if os.IsNotExist(err) {
		h.w.WriteHeader(http.StatusNotFound)
		return
	}

	h.w.Header().Set("Content-Type", contentType)
	h.w.Header().Set("Content-Length", fmt.Sprintf("%d", fi.Size()))
	h.w.Header().Set("Last-Modified", fi.ModTime().Format(http.TimeFormat))
	http.ServeFile(h.w, h.r, reqFile)
}

type route struct {
	reg     *regexp.Regexp
	method  string
	handler func(serviceHandler)
}

var routes = []route{
	{regexp.MustCompile("(.*?)/git-upload-pack$"), "POST", serviceUploadPack},
	{regexp.MustCompile("(.*?)/git-receive-pack$"), "POST", serviceReceivePack},
	{regexp.MustCompile("(.*?)/info/refs$"), "GET", getInfoRefs},
	{regexp.MustCompile("(.*?)/HEAD$"), "GET", getTextFile},
	{regexp.MustCompile("(.*?)/objects/info/alternates$"), "GET", getTextFile},
	{regexp.MustCompile("(.*?)/objects/info/http-alternates$"), "GET", getTextFile},
	{regexp.MustCompile("(.*?)/objects/info/packs$"), "GET", getInfoPacks},
	{regexp.MustCompile("(.*?)/objects/info/[^/]*$"), "GET", getTextFile},
	{regexp.MustCompile("(.*?)/objects/[0-9a-f]{2}/[0-9a-f]{38}$"), "GET", getLooseObject},
	{regexp.MustCompile("(.*?)/objects/pack/pack-[0-9a-f]{40}\\.pack$"), "GET", getPackFile},
	{regexp.MustCompile("(.*?)/objects/pack/pack-[0-9a-f]{40}\\.idx$"), "GET", getIdxFile},
}

// FIXME: use process module
func gitCommand(dir string, args ...string) []byte {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		log.GitLogger.Error(4, fmt.Sprintf("%v - %s", err, out))
	}
	return out
}

func getGitConfig(option, dir string) string {
	out := string(gitCommand(dir, "config", option))
	return out[0 : len(out)-1]
}

func getConfigSetting(service, dir string) bool {
	service = strings.Replace(service, "-", "", -1)
	setting := getGitConfig("http."+service, dir)

	if service == "uploadpack" {
		return setting != "false"
	}

	return setting == "true"
}

func hasAccess(service string, h serviceHandler, checkContentType bool) bool {
	if checkContentType {
		if h.r.Header.Get("Content-Type") != fmt.Sprintf("application/x-git-%s-request", service) {
			return false
		}
	}

	if !(service == "upload-pack" || service == "receive-pack") {
		return false
	}
	if service == "receive-pack" {
		return h.cfg.ReceivePack
	}
	if service == "upload-pack" {
		return h.cfg.UploadPack
	}

	return getConfigSetting(service, h.dir)
}

func serviceRPC(h serviceHandler, service string) {
	defer h.r.Body.Close()

	if !hasAccess(service, h, true) {
		h.w.WriteHeader(http.StatusUnauthorized)
		return
	}

	h.w.Header().Set("Content-Type", fmt.Sprintf("application/x-git-%s-result", service))

	var err error
	var reqBody = h.r.Body

	// Handle GZIP.
	if h.r.Header.Get("Content-Encoding") == "gzip" {
		reqBody, err = gzip.NewReader(reqBody)
		if err != nil {
			log.GitLogger.Error(2, "fail to create gzip reader: %v", err)
			h.w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	// set this for allow pre-receive and post-receive execute
	h.environ = append(h.environ, "SSH_ORIGINAL_COMMAND="+service)

	var stderr bytes.Buffer
	cmd := exec.Command("git", service, "--stateless-rpc", h.dir)
	cmd.Dir = h.dir
	if service == "receive-pack" {
		cmd.Env = append(os.Environ(), h.environ...)
	}
	cmd.Stdout = h.w
	cmd.Stdin = reqBody
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.GitLogger.Error(2, "fail to serve RPC(%s): %v - %v", service, err, stderr)
		return
	}
}

func serviceUploadPack(h serviceHandler) {
	serviceRPC(h, "upload-pack")
}

func serviceReceivePack(h serviceHandler) {
	serviceRPC(h, "receive-pack")
}

func getServiceType(r *http.Request) string {
	serviceType := r.FormValue("service")
	if !strings.HasPrefix(serviceType, "git-") {
		return ""
	}
	return strings.Replace(serviceType, "git-", "", 1)
}

func updateServerInfo(dir string) []byte {
	return gitCommand(dir, "update-server-info")
}

func packetWrite(str string) []byte {
	s := strconv.FormatInt(int64(len(str)+4), 16)
	if len(s)%4 != 0 {
		s = strings.Repeat("0", 4-len(s)%4) + s
	}
	return []byte(s + str)
}

func getInfoRefs(h serviceHandler) {
	h.setHeaderNoCache()
	if hasAccess(getServiceType(h.r), h, false) {
		service := getServiceType(h.r)
		refs := gitCommand(h.dir, service, "--stateless-rpc", "--advertise-refs", ".")

		h.w.Header().Set("Content-Type", fmt.Sprintf("application/x-git-%s-advertisement", service))
		h.w.WriteHeader(http.StatusOK)
		h.w.Write(packetWrite("# service=git-" + service + "\n"))
		h.w.Write([]byte("0000"))
		h.w.Write(refs)
	} else {
		updateServerInfo(h.dir)
		h.sendFile("text/plain; charset=utf-8")
	}
}

func getTextFile(h serviceHandler) {
	h.setHeaderNoCache()
	h.sendFile("text/plain")
}

func getInfoPacks(h serviceHandler) {
	h.setHeaderCacheForever()
	h.sendFile("text/plain; charset=utf-8")
}

func getLooseObject(h serviceHandler) {
	h.setHeaderCacheForever()
	h.sendFile("application/x-git-loose-object")
}

func getPackFile(h serviceHandler) {
	h.setHeaderCacheForever()
	h.sendFile("application/x-git-packed-objects")
}

func getIdxFile(h serviceHandler) {
	h.setHeaderCacheForever()
	h.sendFile("application/x-git-packed-objects-toc")
}

func getGitRepoPath(subdir string) (string, error) {
	if !strings.HasSuffix(subdir, ".git") {
		subdir += ".git"
	}

	fpath := path.Join(setting.RepoRootPath, subdir)
	if _, err := os.Stat(fpath); os.IsNotExist(err) {
		return "", err
	}

	return fpath, nil
}

// HTTPBackend middleware for git smart HTTP protocol
func HTTPBackend(ctx *context.Context, cfg *serviceConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, route := range routes {
			r.URL.Path = strings.ToLower(r.URL.Path) // blue: In case some repo name has upper case name
			if m := route.reg.FindStringSubmatch(r.URL.Path); m != nil {
				if setting.Repository.DisableHTTPGit {
					w.WriteHeader(http.StatusForbidden)
					w.Write([]byte("Interacting with repositories by HTTP protocol is not allowed"))
					return
				}
				if route.method != r.Method {
					if r.Proto == "HTTP/1.1" {
						w.WriteHeader(http.StatusMethodNotAllowed)
						w.Write([]byte("Method Not Allowed"))
					} else {
						w.WriteHeader(http.StatusBadRequest)
						w.Write([]byte("Bad Request"))
					}
					return
				}

				file := strings.Replace(r.URL.Path, m[1]+"/", "", 1)
				dir, err := getGitRepoPath(m[1])
				if err != nil {
					log.GitLogger.Error(4, err.Error())
					ctx.Handle(http.StatusNotFound, "HTTPBackend", err)
					return
				}

				route.handler(serviceHandler{cfg, w, r, dir, file, cfg.Env})
				return
			}
		}

		ctx.Handle(http.StatusNotFound, "HTTPBackend", nil)
		return
	}
}
