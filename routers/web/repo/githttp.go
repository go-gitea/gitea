// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/log"
	repo_module "gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	repo_service "gitea.dev/services/repository"

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

// httpBase does the common work for git http services,
// including early response, authentication, repository lookup and permission check.
func httpBase(ctx *context.Context, optGitService ...string) *serviceHandler {
	if ctx.FormString("go-get") == "1" {
		context.EarlyResponseForGoGetMeta(ctx)
		return nil
	}

	var serviceType string
	var isPull, receivePack bool
	switch util.OptionalArg(optGitService) {
	case "git-receive-pack":
		serviceType = ServiceTypeReceivePack
		receivePack = true
	case "git-upload-pack":
		serviceType = ServiceTypeUploadPack
		isPull = true
	case "git-upload-archive":
		serviceType = ServiceTypeUploadArchive
		isPull = true
	case "":
		isPull = ctx.Req.Method == http.MethodHead || ctx.Req.Method == http.MethodGet
	default: // unknown service
		ctx.Resp.WriteHeader(http.StatusBadRequest)
		return nil
	}

	var accessMode perm.AccessMode
	if isPull {
		accessMode = perm.AccessModeRead
	} else {
		accessMode = perm.AccessModeWrite
	}

	isWiki := false
	unitType := unit.TypeCode
	repoName := strings.TrimSuffix(ctx.PathParam("reponame"), ".git")
	if strings.HasSuffix(repoName, ".wiki") {
		isWiki = true
		unitType = unit.TypeWiki
		repoName = repoName[:len(repoName)-5]
	}

	owner := ctx.ContextUser
	if !owner.IsOrganization() && !owner.IsActive {
		ctx.PlainText(http.StatusForbidden, "Repository cannot be accessed. You cannot push or open issues/pull-requests.")
		return nil
	}

	repoExist := true
	repo, err := repo_model.GetRepositoryByName(ctx, owner.ID, repoName)
	if err != nil {
		if !repo_model.IsErrRepoNotExist(err) {
			ctx.ServerError("GetRepositoryByName", err)
			return nil
		}

		if redirectRepoID, err := repo_model.LookupRedirect(ctx, owner.ID, repoName); err == nil {
			context.RedirectToRepo(ctx.Base, redirectRepoID)
			return nil
		}
		repoExist = false
	}

	// Don't allow pushing if the repo is archived
	if repoExist && repo.IsArchived && !isPull {
		ctx.PlainText(http.StatusForbidden, "This repo is archived. You can view files and clone it, but cannot push or open issues/pull-requests.")
		return nil
	}

	// Only public pulls don't need auth: repo must exist, not require-sign-in
	canAnonymousPull := false
	if isPull && repoExist && !setting.Service.RequireSignInViewStrict {
		// allow anonymous pulls if owner is public and repo is public (not private)
		if owner.Visibility == structs.VisibleTypePublic && !repo.IsPrivate {
			canAnonymousPull = true
		}
		// then check "public anonymous access" permission
		if !canAnonymousPull && ctx.Doer == nil {
			anonPerm, err := access_model.GetDoerRepoPermission(ctx, repo, nil)
			if err != nil {
				ctx.ServerError("GetDoerRepoPermission", err)
				return nil
			}
			canAnonymousPull = anonPerm.CanAccess(accessMode, unitType)
		}
	}

	// check access
	if !canAnonymousPull { // not public pull, then either the pull needs auth, or the push needs "write" permission, so ask auth
		if !ctx.IsSigned {
			// TODO: support digit auth - which would be Authorization header with digit
			if setting.OAuth2.Enabled {
				// `Basic realm="Gitea"` tells the GCM to use builtin OAuth2 application: https://github.com/git-ecosystem/git-credential-manager/pull/1442
				ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="Gitea"`)
			} else {
				// If OAuth2 is disabled, then use another realm to avoid GCM OAuth2 attempt
				ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="Gitea (Basic Auth)"`)
			}
			ctx.HTTPError(http.StatusUnauthorized)
			return nil
		}

		context.CheckRepoScopedToken(ctx, repo, auth_model.GetScopeLevelFromAccessMode(accessMode))
		if ctx.Written() {
			return nil
		}

		if ctx.IsBasicAuth && ctx.Data["ApiTokenScope"] == nil && ctx.Doer.IsIndividual() {
			_, err = auth_model.GetTwoFactorByUID(ctx, ctx.Doer.ID)
			if err == nil {
				// TODO: This response should be changed to "invalid credentials" for security reasons once the expectation behind it (creating an app token to authenticate) is properly documented
				ctx.PlainText(http.StatusUnauthorized, "Users with two-factor authentication enabled cannot perform HTTP/HTTPS operations via plain username and password. Please create and use a personal access token on the user settings page")
				return nil
			} else if !auth_model.IsErrTwoFactorNotEnrolled(err) {
				ctx.ServerError("IsErrTwoFactorNotEnrolled", err)
				return nil
			}
		}

		if !ctx.Doer.IsActive || ctx.Doer.ProhibitLogin {
			ctx.PlainText(http.StatusForbidden, "Your account is disabled.")
			return nil
		}

		if repoExist {
			// Only the main code repo accepts refs/for pushes, so wiki pushes must keep write checks.
			if git.DefaultFeatures().SupportProcReceive && !isWiki {
				accessMode = perm.AccessModeRead
			}

			p, err := access_model.GetDoerRepoPermission(ctx, repo, ctx.Doer)
			if err != nil {
				ctx.ServerError("GetDoerRepoPermission", err)
				return nil
			}

			if !p.CanAccess(accessMode, unitType) {
				ctx.PlainText(http.StatusNotFound, "Repository not found")
				return nil
			}

			if !isPull && repo.IsMirror {
				ctx.PlainText(http.StatusForbidden, "mirror repository is read-only")
				return nil
			}
		}
	}

	if !repoExist {
		if !receivePack {
			ctx.PlainText(http.StatusNotFound, "Repository not found")
			return nil
		}

		if isWiki { // you cannot send wiki operation before create the repository
			ctx.PlainText(http.StatusNotFound, "Repository not found")
			return nil
		}

		if owner.IsOrganization() && !setting.Repository.EnablePushCreateOrg {
			ctx.PlainText(http.StatusForbidden, "Push to create is not enabled for organizations.")
			return nil
		}
		if !owner.IsOrganization() && !setting.Repository.EnablePushCreateUser {
			ctx.PlainText(http.StatusForbidden, "Push to create is not enabled for users.")
			return nil
		}

		// Return dummy payload if GET receive-pack
		if ctx.Req.Method == http.MethodGet {
			dummyInfoRefs(ctx)
			return nil
		}

		repo, err = repo_service.PushCreateRepo(ctx, ctx.Doer, owner, repoName)
		if err != nil {
			log.Debug("PushCreateRepo: %v", err)
			ctx.Status(http.StatusNotFound) // TODO: need to refactor PushCreateRepo and its returned errors
			return nil
		}
	}

	if isWiki {
		// Ensure the wiki is enabled before we allow access to it
		if _, err := repo.GetUnit(ctx, unit.TypeWiki); err != nil {
			if repo_model.IsErrUnitTypeNotExist(err) {
				ctx.PlainText(http.StatusForbidden, "repository wiki is disabled")
				return nil
			}
			ctx.ServerError("GetUnit(UnitTypeWiki) for "+repo.FullName(), err)
			return nil
		}
	}

	var environ []string
	if !isPull {
		environ = repo_module.DoerPushingEnvironment(ctx.Doer, repo, isWiki)
	}

	return &serviceHandler{serviceType, repo, isWiki, environ}
}

var (
	infoRefsCache []byte
	infoRefsOnce  sync.Once
)

func dummyInfoRefs(ctx *context.Context) {
	infoRefsOnce.Do(func() {
		tmpEmptyRepoDir, cleanup, err := setting.AppDataTempDir("git-repo-content").MkdirTempRandom("gitea-info-refs-cache")
		if err != nil {
			log.Error("Failed to create temp dir for git-receive-pack cache: %v", err)
			return
		}
		defer cleanup()

		if err := git.InitRepositoryLocal(ctx, tmpEmptyRepoDir, true, git.Sha1ObjectFormat.Name()); err != nil {
			log.Error("Failed to init bare repo for git-receive-pack cache: %v", err)
			return
		}

		refs, _, err := gitcmd.NewCommand("receive-pack", "--stateless-rpc", "--advertise-refs", ".").
			WithDir(tmpEmptyRepoDir).
			RunStdBytes(ctx)
		if err != nil {
			log.Error("Failed to prepare git-receive-pack cache: %v", err)
		}

		log.Debug("populating infoRefsCache: \n%s", string(refs))
		infoRefsCache = refs
	})

	ctx.RespHeader().Set("Expires", "Fri, 01 Jan 1980 00:00:00 GMT")
	ctx.RespHeader().Set("Pragma", "no-cache")
	ctx.RespHeader().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
	ctx.RespHeader().Set("Content-Type", "application/x-git-receive-pack-advertisement")
	_ = pktLineWriteText(ctx.Resp, "# service=git-receive-pack")
	_ = pktLineWriteFlush(ctx.Resp)
	_, _ = ctx.Resp.Write(infoRefsCache)
}

type serviceHandler struct {
	serviceType string

	repo    *repo_model.Repository
	isWiki  bool
	environ []string
}

func (h *serviceHandler) getStorageRepo() git.RepositoryFacade {
	if h.isWiki {
		return h.repo.WikiStorageRepo()
	}
	return h.repo.CodeStorageRepo()
}

func setHeaderNoCache(ctx *context.Context) {
	ctx.Resp.Header().Set("Expires", "Fri, 01 Jan 1980 00:00:00 GMT")
	ctx.Resp.Header().Set("Pragma", "no-cache")
	ctx.Resp.Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
}

func setHeaderCacheForever(ctx *context.Context) {
	now := time.Now().Unix()
	expires := now + 365*86400 // 365 days
	ctx.Resp.Header().Set("Date", strconv.FormatInt(now, 10))
	ctx.Resp.Header().Set("Expires", strconv.FormatInt(expires, 10))
	ctx.Resp.Header().Set("Cache-Control", "public, max-age=31536000")
}

func (h *serviceHandler) sendFile(ctx *context.Context, contentType, file string) {
	fs := gitrepo.RepoLocalFS(h.getStorageRepo())
	ctx.Resp.Header().Set("Content-Type", contentType)
	relPath := util.PathJoinRelX(file)
	http.ServeFileFS(ctx.Resp, ctx.Req, fs, relPath)
}

// one or more key=value pairs separated by colons
var safeGitProtocolHeader = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`^[0-9a-zA-Z]+=[0-9a-zA-Z]+(:[0-9a-zA-Z]+=[0-9a-zA-Z]+)*$`)
})

func prepareGitCmdEnvs(ctx *context.Context, h *serviceHandler, more ...string) []string {
	envs := slices.Clone(os.Environ())
	envs = append(envs, h.environ...)
	envs = append(envs, more...)
	if protocol := ctx.Req.Header.Get("Git-Protocol"); protocol != "" && safeGitProtocolHeader().MatchString(protocol) {
		envs = append(envs, "GIT_PROTOCOL="+protocol)
	}
	return envs
}

func prepareGitCmdWithAllowedService(service string, allowedServices []string) *gitcmd.Command {
	if !slices.Contains(allowedServices, service) {
		return nil
	}
	switch service {
	case ServiceTypeReceivePack:
		return gitcmd.NewCommand(ServiceTypeReceivePack)
	case ServiceTypeUploadPack:
		return gitcmd.NewCommand(ServiceTypeUploadPack)
	case ServiceTypeUploadArchive:
		return gitcmd.NewCommand(ServiceTypeUploadArchive)
	default:
		return nil
	}
}

func serviceRPC(ctx *context.Context, service string) {
	defer ctx.Req.Body.Close()
	h := httpBase(ctx, "git-"+service)
	if h == nil {
		return
	}

	expectedContentType := fmt.Sprintf("application/x-git-%s-request", service)
	if ctx.Req.Header.Get("Content-Type") != expectedContentType {
		log.Debug("Content-Type (%q) doesn't match expected: %q", ctx.Req.Header.Get("Content-Type"), expectedContentType)
		ctx.Resp.WriteHeader(http.StatusBadRequest)
		return
	}

	cmd := prepareGitCmdWithAllowedService(service, []string{ServiceTypeUploadPack, ServiceTypeReceivePack, ServiceTypeUploadArchive})
	if cmd == nil {
		ctx.Resp.WriteHeader(http.StatusBadRequest)
		return
	}
	// git upload-archive does not have a "--stateless-rpc" option
	if service == ServiceTypeUploadPack || service == ServiceTypeReceivePack {
		cmd.AddArguments("--stateless-rpc")
	}

	ctx.Resp.Header().Set("Content-Type", fmt.Sprintf("application/x-git-%s-result", service))

	reqBody := ctx.Req.Body

	// Handle GZIP.
	if ctx.Req.Header.Get("Content-Encoding") == "gzip" {
		var err error
		reqBody, err = gzip.NewReader(reqBody)
		if err != nil {
			ctx.Resp.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// set SSH_ORIGINAL_COMMAND to allow pre-receive and post-receive hooks
	gitCmdEnvs := prepareGitCmdEnvs(ctx, h, "SSH_ORIGINAL_COMMAND="+service)
	err := cmd.AddArguments(".").
		WithRepo(h.getStorageRepo()).WithEnv(gitCmdEnvs).
		WithStdinCopy(reqBody).
		WithStdoutCopy(ctx.Resp).
		RunWithStderr(ctx)
	if err != nil && !gitcmd.IsErrorCanceledOrKilled(err) && !httplib.IsClientOrNetworkError(ctx, err) {
		log.Error("Fail to serve RPC(%s) for repo %s: %v", service, h.getStorageRepo().LogString(), err)
	}
}

const (
	ServiceTypeUploadPack    = "upload-pack"
	ServiceTypeReceivePack   = "receive-pack"
	ServiceTypeUploadArchive = "upload-archive"
)

// ServiceUploadPack implements Git Smart HTTP protocol
func ServiceUploadPack(ctx *context.Context) {
	serviceRPC(ctx, ServiceTypeUploadPack)
}

// ServiceReceivePack implements Git Smart HTTP protocol
func ServiceReceivePack(ctx *context.Context) {
	serviceRPC(ctx, ServiceTypeReceivePack)
}

func ServiceUploadArchive(ctx *context.Context) {
	serviceRPC(ctx, ServiceTypeUploadArchive)
}

func pktLineWriteText(w io.Writer, str string) error {
	// https://git-scm.com/docs/gitprotocol-common
	prefix := strconv.FormatInt(int64(len(str)+4+1), 16)
	if len(prefix)%4 != 0 {
		prefix = "0000" + prefix
		prefix = prefix[len(prefix)-4:]
	}
	if _, err := io.WriteString(w, prefix); err != nil {
		return err
	}
	if _, err := io.WriteString(w, str); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func pktLineWriteFlush(w io.Writer) error {
	_, err := io.WriteString(w, "0000")
	return err
}

// GetInfoRefs implements Git dumb HTTP
// ref: https://git-scm.com/docs/gitprotocol-http , https://git-scm.com/docs/gitprotocol-v2
func GetInfoRefs(ctx *context.Context) {
	h := httpBase(ctx, ctx.FormString("service")) // git http protocol: "?service=git-<service>"
	if h == nil {
		return
	}

	repo := h.getStorageRepo()
	setHeaderNoCache(ctx)

	if h.serviceType == "" {
		// it's said that some legacy git clients will send requests to "/info/refs" without "service" parameter,
		// although there should be no such case client in the modern days. TODO: not quite sure why we need this UpdateServerInfo logic
		if err := git.UpdateServerInfo(ctx, repo); err != nil {
			ctx.ServerError("UpdateServerInfo", err)
			return
		}
		h.sendFile(ctx, "text/plain; charset=utf-8", "info/refs")
		return
	}

	gitCmdEnvs := prepareGitCmdEnvs(ctx, h)
	cmd := prepareGitCmdWithAllowedService(h.serviceType, []string{ServiceTypeUploadPack, ServiceTypeReceivePack})
	if cmd == nil {
		ctx.Resp.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx.Resp.Header().Set("Content-Type", fmt.Sprintf("application/x-git-%s-advertisement", h.serviceType))

	repoExists, err := git.IsRepositoryExist(ctx, repo)
	if err != nil {
		ctx.ServerError("IsRepositoryExist", err)
		return
	}

	if !repoExists {
		ctx.Resp.WriteHeader(http.StatusOK)
		// error-line = PKT-LINE("ERR" SP explanation-text)
		errMsg := "repository doesn't exist"
		if h.isWiki {
			errMsg = "wiki doesn't exist, please initialize the wiki by creating a new page first"
		}
		_ = pktLineWriteText(ctx.Resp, "ERR "+errMsg)
		return
	}

	cmd = cmd.AddArguments("--stateless-rpc", "--advertise-refs", ".").WithEnv(gitCmdEnvs)
	refs, _, err := cmd.WithRepo(repo).RunStdBytes(ctx)
	if err != nil {
		ctx.ServerError("RunGitServiceAdvertiseRefs", err)
		return
	}

	// https://git-scm.com/docs/gitprotocol-pack
	ctx.Resp.WriteHeader(http.StatusOK)
	_ = pktLineWriteText(ctx.Resp, "# service=git-"+h.serviceType)
	_ = pktLineWriteFlush(ctx.Resp)
	_, _ = ctx.Resp.Write(refs)
}

// GetTextFile implements Git dumb HTTP
func GetTextFile(p string) func(*context.Context) {
	return func(ctx *context.Context) {
		h := httpBase(ctx)
		if h != nil {
			setHeaderNoCache(ctx)
			file := ctx.PathParam("file")
			if file != "" {
				h.sendFile(ctx, "text/plain", "objects/info/"+file)
			} else {
				h.sendFile(ctx, "text/plain", p)
			}
		}
	}
}

// GetInfoPacks implements Git dumb HTTP
func GetInfoPacks(ctx *context.Context) {
	h := httpBase(ctx)
	if h != nil {
		setHeaderCacheForever(ctx)
		h.sendFile(ctx, "text/plain; charset=utf-8", "objects/info/packs")
	}
}

// GetLooseObject implements Git dumb HTTP
func GetLooseObject(ctx *context.Context) {
	h := httpBase(ctx)
	if h != nil {
		setHeaderCacheForever(ctx)
		h.sendFile(ctx, "application/x-git-loose-object", fmt.Sprintf("objects/%s/%s",
			ctx.PathParam("head"), ctx.PathParam("hash")))
	}
}

// GetPackFile implements Git dumb HTTP
func GetPackFile(ctx *context.Context) {
	h := httpBase(ctx)
	if h != nil {
		setHeaderCacheForever(ctx)
		h.sendFile(ctx, "application/x-git-packed-objects", "objects/pack/pack-"+ctx.PathParam("file")+".pack")
	}
}

// GetIdxFile implements Git dumb HTTP
func GetIdxFile(ctx *context.Context) {
	h := httpBase(ctx)
	if h != nil {
		setHeaderCacheForever(ctx)
		h.sendFile(ctx, "application/x-git-packed-objects-toc", "objects/pack/pack-"+ctx.PathParam("file")+".idx")
	}
}
