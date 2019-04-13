// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/go-macaron/csrf"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/paginater"
	macaron "gopkg.in/macaron.v1"
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
		URL:     base.DocURL,
	})
}

// SetLinkHeader sets pagination link header by given total number and page size.
func (ctx *APIContext) SetLinkHeader(total, pageSize int) {
	page := paginater.New(total, pageSize, ctx.QueryInt("page"), 0)
	links := make([]string, 0, 4)
	if page.HasNext() {
		links = append(links, fmt.Sprintf("<%s%s?page=%d>; rel=\"next\"", setting.AppURL, ctx.Req.URL.Path[1:], page.Next()))
	}
	if !page.IsLast() {
		links = append(links, fmt.Sprintf("<%s%s?page=%d>; rel=\"last\"", setting.AppURL, ctx.Req.URL.Path[1:], page.TotalPages()))
	}
	if !page.IsFirst() {
		links = append(links, fmt.Sprintf("<%s%s?page=1>; rel=\"first\"", setting.AppURL, ctx.Req.URL.Path[1:]))
	}
	if page.HasPrevious() {
		links = append(links, fmt.Sprintf("<%s%s?page=%d>; rel=\"prev\"", setting.AppURL, ctx.Req.URL.Path[1:], page.Previous()))
	}

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
func ReferencesGitRepo() macaron.Handler {
	return func(ctx *APIContext) {
		// Empty repository does not have reference information.
		if ctx.Repo.Repository.IsEmpty {
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
		}
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

	u, err := url.Parse(setting.AppURL)
	if err != nil {
		ctx.Error(500, "Invalid AppURL", err)
		return
	}
	u.Path = path.Join(u.Path, "api", "swagger")
	ctx.JSON(404, map[string]interface{}{
		"message":           message,
		"documentation_url": u.String(),
		"errors":            errors,
	})
}
