// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	mc "code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	web_types "code.gitea.io/gitea/modules/web/types"

	"gitea.com/go-chi/cache"
)

// APIContext is a specific context for API service
type APIContext struct {
	*Base

	Cache cache.Cache

	Doer        *user_model.User // current signed-in user
	IsSigned    bool
	IsBasicAuth bool

	ContextUser *user_model.User // the user which is being visited, in most cases it differs from Doer

	Repo    *Repository
	Org     *APIOrganization
	Package *Package
}

func init() {
	web.RegisterResponseStatusProvider[*APIContext](func(req *http.Request) web_types.ResponseStatusProvider {
		return req.Context().Value(apiContextKey).(*APIContext)
	})
}

// Currently, we have the following common fields in error response:
// * message: the message for end users (it shouldn't be used for error type detection)
//            if we need to indicate some errors, we should introduce some new fields like ErrorCode or ErrorType
// * url:     the swagger document URL

// APIError is error format response
// swagger:response error
type APIError struct {
	Message string `json:"message"`
	URL     string `json:"url"`
}

// APIValidationError is error format response related to input validation
// swagger:response validationError
type APIValidationError struct {
	Message string `json:"message"`
	URL     string `json:"url"`
}

// APIInvalidTopicsError is error format response to invalid topics
// swagger:response invalidTopicsError
type APIInvalidTopicsError struct {
	Message       string   `json:"message"`
	InvalidTopics []string `json:"invalidTopics"`
}

// APIEmpty is an empty response
// swagger:response empty
type APIEmpty struct{}

// APIForbiddenError is a forbidden error response
// swagger:response forbidden
type APIForbiddenError struct {
	APIError
}

// APINotFound is a not found empty response
// swagger:response notFound
type APINotFound struct{}

// APIConflict is a conflict empty response
// swagger:response conflict
type APIConflict struct{}

// APIRedirect is a redirect response
// swagger:response redirect
type APIRedirect struct{}

// APIString is a string response
// swagger:response string
type APIString string

// ServerError responds with error message, status is 500
func (ctx *APIContext) ServerError(title string, err error) {
	ctx.Error(http.StatusInternalServerError, title, err)
}

// Error responds with an error message to client with given obj as the message.
// If status is 500, also it prints error to log.
func (ctx *APIContext) Error(status int, title string, obj any) {
	var message string
	if err, ok := obj.(error); ok {
		message = err.Error()
	} else {
		message = fmt.Sprintf("%s", obj)
	}

	if status == http.StatusInternalServerError {
		log.ErrorWithSkip(1, "%s: %s", title, message)

		if setting.IsProd && !(ctx.Doer != nil && ctx.Doer.IsAdmin) {
			message = ""
		}
	}

	ctx.JSON(status, APIError{
		Message: message,
		URL:     setting.API.SwaggerURL,
	})
}

// InternalServerError responds with an error message to the client with the error as a message
// and the file and line of the caller.
func (ctx *APIContext) InternalServerError(err error) {
	log.ErrorWithSkip(1, "InternalServerError: %v", err)

	var message string
	if !setting.IsProd || (ctx.Doer != nil && ctx.Doer.IsAdmin) {
		message = err.Error()
	}

	ctx.JSON(http.StatusInternalServerError, APIError{
		Message: message,
		URL:     setting.API.SwaggerURL,
	})
}

type apiContextKeyType struct{}

var apiContextKey = apiContextKeyType{}

// GetAPIContext returns a context for API routes
func GetAPIContext(req *http.Request) *APIContext {
	return req.Context().Value(apiContextKey).(*APIContext)
}

func genAPILinks(curURL *url.URL, total, pageSize, curPage int) []string {
	page := NewPagination(total, pageSize, curPage, 0)
	paginater := page.Paginater
	links := make([]string, 0, 4)

	if paginater.HasNext() {
		u := *curURL
		queries := u.Query()
		queries.Set("page", fmt.Sprintf("%d", paginater.Next()))
		u.RawQuery = queries.Encode()

		links = append(links, fmt.Sprintf("<%s%s>; rel=\"next\"", setting.AppURL, u.RequestURI()[1:]))
	}
	if !paginater.IsLast() {
		u := *curURL
		queries := u.Query()
		queries.Set("page", fmt.Sprintf("%d", paginater.TotalPages()))
		u.RawQuery = queries.Encode()

		links = append(links, fmt.Sprintf("<%s%s>; rel=\"last\"", setting.AppURL, u.RequestURI()[1:]))
	}
	if !paginater.IsFirst() {
		u := *curURL
		queries := u.Query()
		queries.Set("page", "1")
		u.RawQuery = queries.Encode()

		links = append(links, fmt.Sprintf("<%s%s>; rel=\"first\"", setting.AppURL, u.RequestURI()[1:]))
	}
	if paginater.HasPrevious() {
		u := *curURL
		queries := u.Query()
		queries.Set("page", fmt.Sprintf("%d", paginater.Previous()))
		u.RawQuery = queries.Encode()

		links = append(links, fmt.Sprintf("<%s%s>; rel=\"prev\"", setting.AppURL, u.RequestURI()[1:]))
	}
	return links
}

// SetLinkHeader sets pagination link header by given total number and page size.
func (ctx *APIContext) SetLinkHeader(total, pageSize int) {
	links := genAPILinks(ctx.Req.URL, total, pageSize, ctx.FormInt("page"))

	if len(links) > 0 {
		ctx.RespHeader().Set("Link", strings.Join(links, ","))
		ctx.AppendAccessControlExposeHeaders("Link")
	}
}

// CheckForOTP validates OTP
func (ctx *APIContext) CheckForOTP() {
	if skip, ok := ctx.Data["SkipLocalTwoFA"]; ok && skip.(bool) {
		return // Skip 2FA
	}

	otpHeader := ctx.Req.Header.Get("X-Gitea-OTP")
	twofa, err := auth.GetTwoFactorByUID(ctx.Doer.ID)
	if err != nil {
		if auth.IsErrTwoFactorNotEnrolled(err) {
			return // No 2FA enrollment for this user
		}
		ctx.Error(http.StatusInternalServerError, "GetTwoFactorByUID", err)
		return
	}
	ok, err := twofa.ValidateTOTP(otpHeader)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ValidateTOTP", err)
		return
	}
	if !ok {
		ctx.Error(http.StatusUnauthorized, "", nil)
		return
	}
}

// APIContexter returns apicontext as middleware
func APIContexter() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			base, baseCleanUp := NewBaseContext(w, req)
			ctx := &APIContext{
				Base:  base,
				Cache: mc.GetCache(),
				Repo:  &Repository{PullRequest: &PullRequest{}},
				Org:   &APIOrganization{},
			}
			defer baseCleanUp()

			ctx.Base.AppendContextValue(apiContextKey, ctx)
			ctx.Base.AppendContextValueFunc(git.RepositoryContextKey, func() any { return ctx.Repo.GitRepo })

			// If request sends files, parse them here otherwise the Query() can't be parsed and the CsrfToken will be invalid.
			if ctx.Req.Method == "POST" && strings.Contains(ctx.Req.Header.Get("Content-Type"), "multipart/form-data") {
				if err := ctx.Req.ParseMultipartForm(setting.Attachment.MaxSize << 20); err != nil && !strings.Contains(err.Error(), "EOF") { // 32MB max size
					ctx.InternalServerError(err)
					return
				}
			}

			httpcache.SetCacheControlInHeader(ctx.Resp.Header(), 0, "no-transform")
			ctx.Resp.Header().Set(`X-Frame-Options`, setting.CORSConfig.XFrameOptions)

			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}

// NotFound handles 404s for APIContext
// String will replace message, errors will be added to a slice
func (ctx *APIContext) NotFound(objs ...any) {
	message := ctx.Tr("error.not_found")
	var errors []string
	for _, obj := range objs {
		// Ignore nil
		if obj == nil {
			continue
		}

		if err, ok := obj.(error); ok {
			errors = append(errors, err.Error())
		} else {
			message = obj.(string)
		}
	}

	ctx.JSON(http.StatusNotFound, map[string]any{
		"message": message,
		"url":     setting.API.SwaggerURL,
		"errors":  errors,
	})
}

// ReferencesGitRepo injects the GitRepo into the Context
// you can optional skip the IsEmpty check
func ReferencesGitRepo(allowEmpty ...bool) func(ctx *APIContext) (cancel context.CancelFunc) {
	return func(ctx *APIContext) (cancel context.CancelFunc) {
		// Empty repository does not have reference information.
		if ctx.Repo.Repository.IsEmpty && !(len(allowEmpty) != 0 && allowEmpty[0]) {
			return
		}

		// For API calls.
		if ctx.Repo.GitRepo == nil {
			repoPath := repo_model.RepoPath(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
			gitRepo, err := git.OpenRepository(ctx, repoPath)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "RepoRef Invalid repo "+repoPath, err)
				return
			}
			ctx.Repo.GitRepo = gitRepo
			// We opened it, we should close it
			return func() {
				// If it's been set to nil then assume someone else has closed it.
				if ctx.Repo.GitRepo != nil {
					_ = ctx.Repo.GitRepo.Close()
				}
			}
		}

		return cancel
	}
}

// RepoRefForAPI handles repository reference names when the ref name is not explicitly given
func RepoRefForAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := GetAPIContext(req)

		if ctx.Repo.GitRepo == nil {
			ctx.InternalServerError(fmt.Errorf("no open git repo"))
			return
		}

		if ref := ctx.FormTrim("ref"); len(ref) > 0 {
			commit, err := ctx.Repo.GitRepo.GetCommit(ref)
			if err != nil {
				if git.IsErrNotExist(err) {
					ctx.NotFound()
				} else {
					ctx.Error(http.StatusInternalServerError, "GetCommit", err)
				}
				return
			}
			ctx.Repo.Commit = commit
			ctx.Repo.TreePath = ctx.Params("*")
			next.ServeHTTP(w, req)
			return
		}

		var err error
		refName := getRefName(ctx.Base, ctx.Repo, RepoRefAny)

		if ctx.Repo.GitRepo.IsBranchExist(refName) {
			ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(refName)
			if err != nil {
				ctx.InternalServerError(err)
				return
			}
			ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()
		} else if ctx.Repo.GitRepo.IsTagExist(refName) {
			ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetTagCommit(refName)
			if err != nil {
				ctx.InternalServerError(err)
				return
			}
			ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()
		} else if len(refName) == git.SHAFullLength {
			ctx.Repo.CommitID = refName
			ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetCommit(refName)
			if err != nil {
				ctx.NotFound("GetCommit", err)
				return
			}
		} else {
			ctx.NotFound(fmt.Errorf("not exist: '%s'", ctx.Params("*")))
			return
		}

		next.ServeHTTP(w, req)
	})
}

// HasAPIError returns true if error occurs in form validation.
func (ctx *APIContext) HasAPIError() bool {
	hasErr, ok := ctx.Data["HasError"]
	if !ok {
		return false
	}
	return hasErr.(bool)
}

// GetErrMsg returns error message in form validation.
func (ctx *APIContext) GetErrMsg() string {
	msg, _ := ctx.Data["ErrorMsg"].(string)
	if msg == "" {
		msg = "invalid form data"
	}
	return msg
}

// NotFoundOrServerError use error check function to determine if the error
// is about not found. It responds with 404 status code for not found error,
// or error context description for logging purpose of 500 server error.
func (ctx *APIContext) NotFoundOrServerError(logMsg string, errCheck func(error) bool, logErr error) {
	if errCheck(logErr) {
		ctx.JSON(http.StatusNotFound, nil)
		return
	}
	ctx.Error(http.StatusInternalServerError, "NotFoundOrServerError", logMsg)
}

// IsUserSiteAdmin returns true if current user is a site admin
func (ctx *APIContext) IsUserSiteAdmin() bool {
	return ctx.IsSigned && ctx.Doer.IsAdmin
}

// IsUserRepoAdmin returns true if current user is admin in current repo
func (ctx *APIContext) IsUserRepoAdmin() bool {
	return ctx.Repo.IsAdmin()
}

// IsUserRepoWriter returns true if current user has write privilege in current repo
func (ctx *APIContext) IsUserRepoWriter(unitTypes []unit.Type) bool {
	for _, unitType := range unitTypes {
		if ctx.Repo.CanWrite(unitType) {
			return true
		}
	}

	return false
}
