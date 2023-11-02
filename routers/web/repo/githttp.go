// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"compress/gzip"
	gocontext "context"
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/go-chi/cors"
)

func HTTPGitEnabledHandler(ctx *context.Context) {
	if setting.Repository.DisableHTTPGit {
		ctx.Resp.WriteHeader(http.StatusForbidden)
		_, _ = ctx.Resp.Write([]byte("Interacting with repositories by HTTP protocol is not allowed"))
	}
}

func CorsHandler() func(next http.Handler) http.Handler {
	if setting.Repository.AccessControlAllowOrigin != "" {
		return cors.Handler(cors.Options{
			AllowedOrigins: []string{setting.Repository.AccessControlAllowOrigin},
			AllowedHeaders: []string{"Content-Type", "Authorization", "User-Agent"},
		})
	}
	return func(next http.Handler) http.Handler {
		return next
	}
}

// httpBase implementation git smart HTTP protocol
func httpBase(ctx *context.Context) (h *serviceHandler) {
	username := ctx.Params(":username")
	reponame := strings.TrimSuffix(ctx.Params(":reponame"), ".git")

	if ctx.FormString("go-get") == "1" {
		context.EarlyResponseForGoGetMeta(ctx)
		return
	}

	var isPull, receivePack bool
	service := ctx.FormString("service")
	if service == "git-receive-pack" ||
		strings.HasSuffix(ctx.Req.URL.Path, "git-receive-pack") {
		isPull = false
		receivePack = true
	} else if service == "git-upload-pack" ||
		strings.HasSuffix(ctx.Req.URL.Path, "git-upload-pack") {
		isPull = true
	} else if service == "git-upload-archive" ||
		strings.HasSuffix(ctx.Req.URL.Path, "git-upload-archive") {
		isPull = true
	} else {
		isPull = ctx.Req.Method == "GET"
	}

	var accessMode perm.AccessMode
	if isPull {
		accessMode = perm.AccessModeRead
	} else {
		accessMode = perm.AccessModeWrite
	}

	isWiki := false
	unitType := unit.TypeCode
	var wikiRepoName string
	if strings.HasSuffix(reponame, ".wiki") {
		isWiki = true
		unitType = unit.TypeWiki
		wikiRepoName = reponame
		reponame = reponame[:len(reponame)-5]
	}

	owner := ctx.ContextUser
	if !owner.IsOrganization() && !owner.IsActive {
		ctx.PlainText(http.StatusForbidden, "Repository cannot be accessed. You cannot push or open issues/pull-requests.")
		return
	}

	repoExist := true
	repo, err := repo_model.GetRepositoryByName(owner.ID, reponame)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			if redirectRepoID, err := repo_model.LookupRedirect(owner.ID, reponame); err == nil {
				context.RedirectToRepo(ctx.Base, redirectRepoID)
				return
			}
			repoExist = false
		} else {
			ctx.ServerError("GetRepositoryByName", err)
			return
		}
	}

	// Don't allow pushing if the repo is archived
	if repoExist && repo.IsArchived && !isPull {
		ctx.PlainText(http.StatusForbidden, "This repo is archived. You can view files and clone it, but cannot push or open issues/pull-requests.")
		return
	}

	// Only public pull don't need auth.
	isPublicPull := repoExist && !repo.IsPrivate && isPull
	var (
		askAuth = !isPublicPull || setting.Service.RequireSignInView
		environ []string
	)

	// don't allow anonymous pulls if organization is not public
	if isPublicPull {
		if err := repo.LoadOwner(ctx); err != nil {
			ctx.ServerError("LoadOwner", err)
			return
		}

		askAuth = askAuth || (repo.Owner.Visibility != structs.VisibleTypePublic)
	}

	// check access
	if askAuth {
		// rely on the results of Contexter
		if !ctx.IsSigned {
			// TODO: support digit auth - which would be Authorization header with digit
			ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=\".\"")
			ctx.Error(http.StatusUnauthorized)
			return
		}

		context.CheckRepoScopedToken(ctx, repo, auth_model.GetScopeLevelFromAccessMode(accessMode))
		if ctx.Written() {
			return
		}

		if ctx.IsBasicAuth && ctx.Data["IsApiToken"] != true && ctx.Data["IsActionsToken"] != true {
			_, err = auth_model.GetTwoFactorByUID(ctx.Doer.ID)
			if err == nil {
				// TODO: This response should be changed to "invalid credentials" for security reasons once the expectation behind it (creating an app token to authenticate) is properly documented
				ctx.PlainText(http.StatusUnauthorized, "Users with two-factor authentication enabled cannot perform HTTP/HTTPS operations via plain username and password. Please create and use a personal access token on the user settings page")
				return
			} else if !auth_model.IsErrTwoFactorNotEnrolled(err) {
				ctx.ServerError("IsErrTwoFactorNotEnrolled", err)
				return
			}
		}

		if !ctx.Doer.IsActive || ctx.Doer.ProhibitLogin {
			ctx.PlainText(http.StatusForbidden, "Your account is disabled.")
			return
		}

		environ = []string{
			repo_module.EnvRepoUsername + "=" + username,
			repo_module.EnvRepoName + "=" + reponame,
			repo_module.EnvPusherName + "=" + ctx.Doer.Name,
			repo_module.EnvPusherID + fmt.Sprintf("=%d", ctx.Doer.ID),
			repo_module.EnvAppURL + "=" + setting.AppURL,
		}

		if repoExist {
			// Because of special ref "refs/for" .. , need delay write permission check
			if git.SupportProcReceive {
				accessMode = perm.AccessModeRead
			}

			if ctx.Data["IsActionsToken"] == true {
				taskID := ctx.Data["ActionsTaskID"].(int64)
				task, err := actions_model.GetTaskByID(ctx, taskID)
				if err != nil {
					ctx.ServerError("GetTaskByID", err)
					return
				}
				if task.RepoID != repo.ID {
					ctx.PlainText(http.StatusForbidden, "User permission denied")
					return
				}

				if task.IsForkPullRequest {
					if accessMode > perm.AccessModeRead {
						ctx.PlainText(http.StatusForbidden, "User permission denied")
						return
					}
					environ = append(environ, fmt.Sprintf("%s=%d", repo_module.EnvActionPerm, perm.AccessModeRead))
				} else {
					if accessMode > perm.AccessModeWrite {
						ctx.PlainText(http.StatusForbidden, "User permission denied")
						return
					}
					environ = append(environ, fmt.Sprintf("%s=%d", repo_module.EnvActionPerm, perm.AccessModeWrite))
				}
			} else {
				p, err := access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
				if err != nil {
					ctx.ServerError("GetUserRepoPermission", err)
					return
				}

				if !p.CanAccess(accessMode, unitType) {
					ctx.PlainText(http.StatusNotFound, "Repository not found")
					return
				}
			}

			if !isPull && repo.IsMirror {
				ctx.PlainText(http.StatusForbidden, "mirror repository is read-only")
				return
			}
		}

		if !ctx.Doer.KeepEmailPrivate {
			environ = append(environ, repo_module.EnvPusherEmail+"="+ctx.Doer.Email)
		}

		if isWiki {
			environ = append(environ, repo_module.EnvRepoIsWiki+"=true")
		} else {
			environ = append(environ, repo_module.EnvRepoIsWiki+"=false")
		}
	}

	if !repoExist {
		if !receivePack {
			ctx.PlainText(http.StatusNotFound, "Repository not found")
			return
		}

		if isWiki { // you cannot send wiki operation before create the repository
			ctx.PlainText(http.StatusNotFound, "Repository not found")
			return
		}

		if owner.IsOrganization() && !setting.Repository.EnablePushCreateOrg {
			ctx.PlainText(http.StatusForbidden, "Push to create is not enabled for organizations.")
			return
		}
		if !owner.IsOrganization() && !setting.Repository.EnablePushCreateUser {
			ctx.PlainText(http.StatusForbidden, "Push to create is not enabled for users.")
			return
		}

		// Return dummy payload if GET receive-pack
		if ctx.Req.Method == http.MethodGet {
			dummyInfoRefs(ctx)
			return
		}

		repo, err = repo_service.PushCreateRepo(ctx, ctx.Doer, owner, reponame)
		if err != nil {
			log.Error("pushCreateRepo: %v", err)
			ctx.Status(http.StatusNotFound)
			return
		}
	}

	if isWiki {
		// Ensure the wiki is enabled before we allow access to it
		if _, err := repo.GetUnit(ctx, unit.TypeWiki); err != nil {
			if repo_model.IsErrUnitTypeNotExist(err) {
				ctx.PlainText(http.StatusForbidden, "repository wiki is disabled")
				return
			}
			log.Error("Failed to get the wiki unit in %-v Error: %v", repo, err)
			ctx.ServerError("GetUnit(UnitTypeWiki) for "+repo.FullName(), err)
			return
		}
	}

	environ = append(environ, repo_module.EnvRepoID+fmt.Sprintf("=%d", repo.ID))

	w := ctx.Resp
	r := ctx.Req
	cfg := &serviceConfig{
		UploadPack:  true,
		ReceivePack: true,
		Env:         environ,
	}

	r.URL.Path = strings.ToLower(r.URL.Path) // blue: In case some repo name has upper case name

	dir := repo_model.RepoPath(username, reponame)
	if isWiki {
		dir = repo_model.RepoPath(username, wikiRepoName)
	}

	return &serviceHandler{cfg, w, r, dir, cfg.Env}
}

var (
	infoRefsCache []byte
	infoRefsOnce  sync.Once
)

func dummyInfoRefs(ctx *context.Context) {
	infoRefsOnce.Do(func() {
		tmpDir, err := os.MkdirTemp(os.TempDir(), "gitea-info-refs-cache")
		if err != nil {
			log.Error("Failed to create temp dir for git-receive-pack cache: %v", err)
			return
		}

		defer func() {
			if err := util.RemoveAll(tmpDir); err != nil {
				log.Error("RemoveAll: %v", err)
			}
		}()

		if err := git.InitRepository(ctx, tmpDir, true); err != nil {
			log.Error("Failed to init bare repo for git-receive-pack cache: %v", err)
			return
		}

		refs, _, err := git.NewCommand(ctx, "receive-pack", "--stateless-rpc", "--advertise-refs", ".").RunStdBytes(&git.RunOpts{Dir: tmpDir})
		if err != nil {
			log.Error(fmt.Sprintf("%v - %s", err, string(refs)))
		}

		log.Debug("populating infoRefsCache: \n%s", string(refs))
		infoRefsCache = refs
	})

	ctx.RespHeader().Set("Expires", "Fri, 01 Jan 1980 00:00:00 GMT")
	ctx.RespHeader().Set("Pragma", "no-cache")
	ctx.RespHeader().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
	ctx.RespHeader().Set("Content-Type", "application/x-git-receive-pack-advertisement")
	_, _ = ctx.Write(packetWrite("# service=git-receive-pack\n"))
	_, _ = ctx.Write([]byte("0000"))
	_, _ = ctx.Write(infoRefsCache)
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

func containsParentDirectorySeparator(v string) bool {
	if !strings.Contains(v, "..") {
		return false
	}
	for _, ent := range strings.FieldsFunc(v, isSlashRune) {
		if ent == ".." {
			return true
		}
	}
	return false
}

func isSlashRune(r rune) bool { return r == '/' || r == '\\' }

func (h *serviceHandler) sendFile(contentType, file string) {
	if containsParentDirectorySeparator(file) {
		log.Error("request file path contains invalid path: %v", file)
		h.w.WriteHeader(http.StatusBadRequest)
		return
	}
	reqFile := path.Join(h.dir, file)

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

// one or more key=value pairs separated by colons
var safeGitProtocolHeader = regexp.MustCompile(`^[0-9a-zA-Z]+=[0-9a-zA-Z]+(:[0-9a-zA-Z]+=[0-9a-zA-Z]+)*$`)

func prepareGitCmdWithAllowedService(service string, h *serviceHandler) (*git.Command, error) {
	if service == "receive-pack" && h.cfg.ReceivePack {
		return git.NewCommand(h.r.Context(), "receive-pack"), nil
	}
	if service == "upload-pack" && h.cfg.UploadPack {
		return git.NewCommand(h.r.Context(), "upload-pack"), nil
	}

	return nil, fmt.Errorf("service %q is not allowed", service)
}

func serviceRPC(h *serviceHandler, service string) {
	defer func() {
		if err := h.r.Body.Close(); err != nil {
			log.Error("serviceRPC: Close: %v", err)
		}
	}()

	expectedContentType := fmt.Sprintf("application/x-git-%s-request", service)
	if h.r.Header.Get("Content-Type") != expectedContentType {
		log.Error("Content-Type (%q) doesn't match expected: %q", h.r.Header.Get("Content-Type"), expectedContentType)
		h.w.WriteHeader(http.StatusUnauthorized)
		return
	}

	cmd, err := prepareGitCmdWithAllowedService(service, h)
	if err != nil {
		log.Error("Failed to prepareGitCmdWithService: %v", err)
		h.w.WriteHeader(http.StatusUnauthorized)
		return
	}

	h.w.Header().Set("Content-Type", fmt.Sprintf("application/x-git-%s-result", service))

	reqBody := h.r.Body

	// Handle GZIP.
	if h.r.Header.Get("Content-Encoding") == "gzip" {
		reqBody, err = gzip.NewReader(reqBody)
		if err != nil {
			log.Error("Fail to create gzip reader: %v", err)
			h.w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	// set this for allow pre-receive and post-receive execute
	h.environ = append(h.environ, "SSH_ORIGINAL_COMMAND="+service)

	if protocol := h.r.Header.Get("Git-Protocol"); protocol != "" && safeGitProtocolHeader.MatchString(protocol) {
		h.environ = append(h.environ, "GIT_PROTOCOL="+protocol)
	}

	var stderr bytes.Buffer
	cmd.AddArguments("--stateless-rpc").AddDynamicArguments(h.dir)
	cmd.SetDescription(fmt.Sprintf("%s %s %s [repo_path: %s]", git.GitExecutable, service, "--stateless-rpc", h.dir))
	if err := cmd.Run(&git.RunOpts{
		Dir:               h.dir,
		Env:               append(os.Environ(), h.environ...),
		Stdout:            h.w,
		Stdin:             reqBody,
		Stderr:            &stderr,
		UseContextTimeout: true,
	}); err != nil {
		if err.Error() != "signal: killed" {
			log.Error("Fail to serve RPC(%s) in %s: %v - %s", service, h.dir, err, stderr.String())
		}
		return
	}
}

// ServiceUploadPack implements Git Smart HTTP protocol
func ServiceUploadPack(ctx *context.Context) {
	h := httpBase(ctx)
	if h != nil {
		serviceRPC(h, "upload-pack")
	}
}

// ServiceReceivePack implements Git Smart HTTP protocol
func ServiceReceivePack(ctx *context.Context) {
	h := httpBase(ctx)
	if h != nil {
		serviceRPC(h, "receive-pack")
	}
}

func getServiceType(r *http.Request) string {
	serviceType := r.FormValue("service")
	if !strings.HasPrefix(serviceType, "git-") {
		return ""
	}
	return strings.TrimPrefix(serviceType, "git-")
}

func updateServerInfo(ctx gocontext.Context, dir string) []byte {
	out, _, err := git.NewCommand(ctx, "update-server-info").RunStdBytes(&git.RunOpts{Dir: dir})
	if err != nil {
		log.Error(fmt.Sprintf("%v - %s", err, string(out)))
	}
	return out
}

func packetWrite(str string) []byte {
	s := strconv.FormatInt(int64(len(str)+4), 16)
	if len(s)%4 != 0 {
		s = strings.Repeat("0", 4-len(s)%4) + s
	}
	return []byte(s + str)
}

// GetInfoRefs implements Git dumb HTTP
func GetInfoRefs(ctx *context.Context) {
	h := httpBase(ctx)
	if h == nil {
		return
	}
	h.setHeaderNoCache()
	service := getServiceType(h.r)
	cmd, err := prepareGitCmdWithAllowedService(service, h)
	if err == nil {
		if protocol := h.r.Header.Get("Git-Protocol"); protocol != "" && safeGitProtocolHeader.MatchString(protocol) {
			h.environ = append(h.environ, "GIT_PROTOCOL="+protocol)
		}
		h.environ = append(os.Environ(), h.environ...)

		refs, _, err := cmd.AddArguments("--stateless-rpc", "--advertise-refs", ".").RunStdBytes(&git.RunOpts{Env: h.environ, Dir: h.dir})
		if err != nil {
			log.Error(fmt.Sprintf("%v - %s", err, string(refs)))
		}

		h.w.Header().Set("Content-Type", fmt.Sprintf("application/x-git-%s-advertisement", service))
		h.w.WriteHeader(http.StatusOK)
		_, _ = h.w.Write(packetWrite("# service=git-" + service + "\n"))
		_, _ = h.w.Write([]byte("0000"))
		_, _ = h.w.Write(refs)
	} else {
		updateServerInfo(ctx, h.dir)
		h.sendFile("text/plain; charset=utf-8", "info/refs")
	}
}

// GetTextFile implements Git dumb HTTP
func GetTextFile(p string) func(*context.Context) {
	return func(ctx *context.Context) {
		h := httpBase(ctx)
		if h != nil {
			h.setHeaderNoCache()
			file := ctx.Params("file")
			if file != "" {
				h.sendFile("text/plain", "objects/info/"+file)
			} else {
				h.sendFile("text/plain", p)
			}
		}
	}
}

// GetInfoPacks implements Git dumb HTTP
func GetInfoPacks(ctx *context.Context) {
	h := httpBase(ctx)
	if h != nil {
		h.setHeaderCacheForever()
		h.sendFile("text/plain; charset=utf-8", "objects/info/packs")
	}
}

// GetLooseObject implements Git dumb HTTP
func GetLooseObject(ctx *context.Context) {
	h := httpBase(ctx)
	if h != nil {
		h.setHeaderCacheForever()
		h.sendFile("application/x-git-loose-object", fmt.Sprintf("objects/%s/%s",
			ctx.Params("head"), ctx.Params("hash")))
	}
}

// GetPackFile implements Git dumb HTTP
func GetPackFile(ctx *context.Context) {
	h := httpBase(ctx)
	if h != nil {
		h.setHeaderCacheForever()
		h.sendFile("application/x-git-packed-objects", "objects/pack/pack-"+ctx.Params("file")+".pack")
	}
}

// GetIdxFile implements Git dumb HTTP
func GetIdxFile(ctx *context.Context) {
	h := httpBase(ctx)
	if h != nil {
		h.setHeaderCacheForever()
		h.sendFile("application/x-git-packed-objects-toc", "objects/pack/pack-"+ctx.Params("file")+".idx")
	}
}
