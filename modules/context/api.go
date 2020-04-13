// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"gitea.com/macaron/csrf"
	"gitea.com/macaron/macaron"
)

// APIContext is a specific macaron context for API service
type APIContext struct {
	*Context
	Org *APIOrganization
}

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
	Topics  []string `json:"invalidTopics"`
	Message string   `json:"message"`
}

//APIEmpty is an empty response
// swagger:response empty
type APIEmpty struct{}

//APIForbiddenError is a forbidden error response
// swagger:response forbidden
type APIForbiddenError struct {
	APIError
}

//APINotFound is a not found empty response
// swagger:response notFound
type APINotFound struct{}

//APIRedirect is a redirect response
// swagger:response redirect
type APIRedirect struct{}

// Error responses error message to client with given message.
// If status is 500, also it prints error to log.
func (ctx *APIContext) Error(status int, title string, obj interface{}) {
	var message string
	if err, ok := obj.(error); ok {
		message = err.Error()
	} else {
		message = obj.(string)
	}

	if status == 500 {
		log.Error("%s: %s", title, message)
	}

	ctx.JSON(status, APIError{
		Message: message,
		URL:     setting.API.SwaggerURL,
	})
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
	links := genAPILinks(ctx.Req.URL, total, pageSize, ctx.QueryInt("page"))

	if len(links) > 0 {
		ctx.Header().Set("Link", strings.Join(links, ","))
	}
}

// RequireCSRF requires a validated a CSRF token
func (ctx *APIContext) RequireCSRF() {
	headerToken := ctx.Req.Header.Get(ctx.csrf.GetHeaderName())
	formValueToken := ctx.Req.FormValue(ctx.csrf.GetFormName())
	if len(headerToken) > 0 || len(formValueToken) > 0 {
		csrf.Validate(ctx.Context.Context, ctx.csrf)
	} else {
		ctx.Context.Error(401)
	}
}

// CheckForOTP validateds OTP
func (ctx *APIContext) CheckForOTP() {
	otpHeader := ctx.Req.Header.Get("X-Gitea-OTP")
	twofa, err := models.GetTwoFactorByUID(ctx.Context.User.ID)
	if err != nil {
		if models.IsErrTwoFactorNotEnrolled(err) {
			return // No 2FA enrollment for this user
		}
		ctx.Context.Error(500)
		return
	}
	ok, err := twofa.ValidateTOTP(otpHeader)
	if err != nil {
		ctx.Context.Error(500)
		return
	}
	if !ok {
		ctx.Context.Error(401)
		return
	}
}

// APIContexter returns apicontext as macaron middleware
func APIContexter() macaron.Handler {
	return func(c *Context) {
		ctx := &APIContext{
			Context: c,
		}
		c.Map(ctx)
	}
}

// ReferencesGitRepo injects the GitRepo into the Context
func ReferencesGitRepo(allowEmpty bool) macaron.Handler {
	return func(ctx *APIContext) {
		// Empty repository does not have reference information.
		if !allowEmpty && ctx.Repo.Repository.IsEmpty {
			return
		}

		// For API calls.
		if ctx.Repo.GitRepo == nil {
			repoPath := models.RepoPath(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
			gitRepo, err := git.OpenRepository(repoPath)
			if err != nil {
				ctx.Error(500, "RepoRef Invalid repo "+repoPath, err)
				return
			}
			ctx.Repo.GitRepo = gitRepo
			// We opened it, we should close it
			defer func() {
				// If it's been set to nil then assume someone else has closed it.
				if ctx.Repo.GitRepo != nil {
					ctx.Repo.GitRepo.Close()
				}
			}()
		}

		ctx.Next()
	}
}

// NotFound handles 404s for APIContext
// String will replace message, errors will be added to a slice
func (ctx *APIContext) NotFound(objs ...interface{}) {
	var message = "Not Found"
	var errors []string
	for _, obj := range objs {
		if err, ok := obj.(error); ok {
			errors = append(errors, err.Error())
		} else {
			message = obj.(string)
		}
	}

	ctx.JSON(404, map[string]interface{}{
		"message":           message,
		"documentation_url": setting.API.SwaggerURL,
		"errors":            errors,
	})
}
