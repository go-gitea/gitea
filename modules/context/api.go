// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
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
		log.Error(4, "%s: %s", title, message)
	}

	ctx.JSON(status, map[string]string{
		"message": message,
		"url":     base.DocURL,
	})
}

// SetLinkHeader sets pagination link header by given totol number and page size.
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

// APIContexter returns apicontext as macaron middleware
func APIContexter() macaron.Handler {
	return func(c *Context) {
		ctx := &APIContext{
			Context: c,
		}
		c.Map(ctx)
	}
}

// ExtractOwnerAndRepo returns a handler that populates the `Repo.Owner` and
// `Repo.Repository` fields of an APIContext
func ExtractOwnerAndRepo() macaron.Handler {
	return func(ctx *APIContext) {
		owner, err := models.GetUserByName(ctx.Params(":username"))
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.Error(422, "", err)
			} else {
				ctx.Error(500, "GetUserByName", err)
			}
			return
		}

		repo, err := models.GetRepositoryByName(owner.ID, ctx.Params(":reponame"))
		if err != nil {
			if models.IsErrRepoNotExist(err) {
				ctx.Status(404)
			} else {
				ctx.Error(500, "GetRepositoryByName", err)
			}
			return
		}
		ctx.Repo.Owner = owner
		ctx.Data["Owner"] = owner
		ctx.Repo.Repository = repo
		ctx.Data["Repository"] = repo
	}
}
