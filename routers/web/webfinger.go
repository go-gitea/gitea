// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

var webfingerRessourcePattern = regexp.MustCompile(`(?i)\A([a-z^:]+):(.*)\z`)

// https://datatracker.ietf.org/doc/html/draft-ietf-appsawg-webfinger-14#section-4.4

type webfingerJRD struct {
	Subject    string                 `json:"subject,omitempty"`
	Aliases    []string               `json:"aliases,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Links      []*webfingerLink       `json:"links,omitempty"`
}

type webfingerLink struct {
	Rel        string                 `json:"rel,omitempty"`
	Type       string                 `json:"type,omitempty"`
	Href       string                 `json:"href,omitempty"`
	Titles     map[string]string      `json:"titles,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// WebfingerQuery returns informations about a resource
// https://datatracker.ietf.org/doc/html/rfc7565
func WebfingerQuery(ctx *context.Context) {
	resource := ctx.FormTrim("resource")

	scheme := "acct"
	uri := resource

	match := webfingerRessourcePattern.FindStringSubmatch(resource)
	if match != nil {
		scheme = match[1]
		uri = match[2]
	}

	appURL, _ := url.Parse(setting.AppURL)

	var u *user_model.User
	var err error

	switch scheme {
	case "acct":
		// allow only the current host
		parts := strings.SplitN(uri, "@", 2)
		if len(parts) != 2 {
			ctx.Error(http.StatusBadRequest)
			return
		}
		if parts[1] != appURL.Host {
			ctx.Error(http.StatusBadRequest)
			return
		}

		u, err = user_model.GetUserByNameCtx(ctx, parts[0])
	case "mailto":
		u, err = user_model.GetUserByEmailContext(ctx, uri)
	default:
		ctx.Error(http.StatusBadRequest)
		return
	}
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Error(http.StatusNotFound)
		} else {
			log.Error("Error getting user: %v", err)
			ctx.Error(http.StatusInternalServerError)
		}
		return
	}

	// Should we check IsUserVisibleToViewer here?

	aliases := make([]string, 0, 1)
	if !u.KeepEmailPrivate {
		aliases = append(aliases, fmt.Sprintf("mailto:%s", u.Email))
	}

	links := []*webfingerLink{
		{
			Rel:  "http://webfinger.net/rel/profile-page",
			Type: "text/html",
			Href: u.HTMLURL(),
		},
		{
			Rel:  "http://webfinger.net/rel/avatar",
			Href: u.AvatarLink(),
		},
	}

	ctx.JSON(http.StatusOK, &webfingerJRD{
		Subject: fmt.Sprintf("acct:%s@%s", url.QueryEscape(u.Name), appURL.Host),
		Aliases: aliases,
		Links:   links,
	})
}
