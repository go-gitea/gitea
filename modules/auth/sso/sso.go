// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sso

import (
	"reflect"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"

	"gitea.com/macaron/macaron"
	"gitea.com/macaron/session"
)

var (
	ssoMethods []SingleSignOn
)

// Methods returns the instances of all registered SSO methods
func Methods() []SingleSignOn {
	return ssoMethods
}

// Register adds the specified instance to the list of available SSO methods
func Register(method SingleSignOn) {
	ssoMethods = append(ssoMethods, method)
}

// Init should be called exactly once when the application starts to allow SSO plugins
// to allocate necessary resources
func Init() {
	for _, method := range Methods() {
		err := method.Init()
		if err != nil {
			log.Error("Could not initialize '%s' SSO method, error: %s", reflect.TypeOf(method).String(), err)
		}
	}
}

// Free should be called exactly once when the application is terminating to allow SSO plugins
// to release necessary resources
func Free() {
	for _, method := range Methods() {
		err := method.Free()
		if err != nil {
			log.Error("Could not free '%s' SSO method, error: %s", reflect.TypeOf(method).String(), err)
		}
	}
}

// SessionUser returns the user object corresponding to the "uid" session variable.
func SessionUser(sess session.Store) *models.User {
	// Get user ID
	uid := sess.Get("uid")
	if uid == nil {
		return nil
	}
	id, ok := uid.(int64)
	if !ok {
		return nil
	}

	// Get user object
	user, err := models.GetUserByID(id)
	if err != nil {
		if !models.IsErrUserNotExist(err) {
			log.Error("GetUserById: %v", err)
		}
		return nil
	}
	return user
}

// isAPIPath returns true if the specified URL is an API path
func isAPIPath(ctx *macaron.Context) bool {
	return strings.HasPrefix(ctx.Req.URL.Path, "/api/")
}

// isAttachmentDownload check if request is a file download (GET) with URL to an attachment
func isAttachmentDownload(ctx *macaron.Context) bool {
	return strings.HasPrefix(ctx.Req.URL.Path, "/attachments/") && ctx.Req.Method == "GET"
}

// init populates the list of SSO authentication plugins in the order they are expected to be
// executed.
//
// The OAuth2 plugin is expected to be executed first, as it must ignore the user id stored
// in the session (if there is a user id stored in session other plugins might return the user
// object for that id).
//
// The Session plugin is expected to be executed second, in order to skip authentication
// for users that have already signed in.
// The SSPI plugin is expected to be executed last as it returns 401 status code if negotiation
// fails or should continue, which would prevent other authentication methods to execute at all.
func init() {
	ssoMethods = []SingleSignOn{
		&OAuth2{},
		&Session{},
		&ReverseProxy{},
		&Basic{},
		&SSPI{},
	}
}
