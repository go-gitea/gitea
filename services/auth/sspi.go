// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/auth/source/sspi"
	gitea_context "code.gitea.io/gitea/services/context"

	gouuid "github.com/google/uuid"
)

const (
	tplSignIn base.TplName = "user/auth/signin"
)

type SSPIAuth interface {
	AppendAuthenticateHeader(w http.ResponseWriter, data string)
	Authenticate(r *http.Request, w http.ResponseWriter) (userInfo *SSPIUserInfo, outToken string, err error)
}

var (
	sspiAuth        SSPIAuth // a global instance of the websspi authenticator to avoid acquiring the server credential handle on every request
	sspiAuthOnce    sync.Once
	sspiAuthErrInit error

	// Ensure the struct implements the interface.
	_ Method = &SSPI{}
)

// SSPI implements the SingleSignOn interface and authenticates requests
// via the built-in SSPI module in Windows for SPNEGO authentication.
// The SSPI plugin is expected to be executed last, as it returns 401 status code if negotiation
// fails (or if negotiation should continue), which would prevent other authentication methods
// to execute at all.
type SSPI struct{}

// Name represents the name of auth method
func (s *SSPI) Name() string {
	return "sspi"
}

// Verify uses SSPI (Windows implementation of SPNEGO) to authenticate the request.
// If authentication is successful, returns the corresponding user object.
// If negotiation should continue or authentication fails, immediately returns a 401 HTTP
// response code, as required by the SPNEGO protocol.
func (s *SSPI) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) (*user_model.User, error) {
	sspiAuthOnce.Do(func() { sspiAuthErrInit = sspiAuthInit() })
	if sspiAuthErrInit != nil {
		return nil, sspiAuthErrInit
	}
	if !s.shouldAuthenticate(req) {
		return nil, nil
	}

	cfg, err := s.getConfig(req.Context())
	if err != nil {
		log.Error("could not get SSPI config: %v", err)
		return nil, err
	}

	log.Trace("SSPI Authorization: Attempting to authenticate")
	userInfo, outToken, err := sspiAuth.Authenticate(req, w)
	if err != nil {
		log.Warn("Authentication failed with error: %v\n", err)
		sspiAuth.AppendAuthenticateHeader(w, outToken)

		// Include the user login page in the 401 response to allow the user
		// to login with another authentication method if SSPI authentication
		// fails
		store.GetData()["Flash"] = map[string]string{
			"ErrorMsg": err.Error(),
		}
		store.GetData()["EnableOpenIDSignIn"] = setting.Service.EnableOpenIDSignIn
		store.GetData()["EnableSSPI"] = true
		// in this case, the Verify function is called in Gitea's web context
		// FIXME: it doesn't look good to render the page here, why not redirect?
		gitea_context.GetWebContext(req.Context()).HTML(http.StatusUnauthorized, tplSignIn)
		return nil, err
	}
	if outToken != "" {
		sspiAuth.AppendAuthenticateHeader(w, outToken)
	}

	username := sanitizeUsername(userInfo.Username, cfg)
	if len(username) == 0 {
		return nil, nil
	}
	log.Info("Authenticated as %s\n", username)

	user, err := user_model.GetUserByName(req.Context(), username)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			log.Error("GetUserByName: %v", err)
			return nil, err
		}
		if !cfg.AutoCreateUsers {
			log.Error("User '%s' not found", username)
			return nil, nil
		}
		user, err = s.newUser(req.Context(), username, cfg)
		if err != nil {
			log.Error("CreateUser: %v", err)
			return nil, err
		}
	}

	// Make sure requests to API paths and PWA resources do not create a new session
	if !middleware.IsAPIPath(req) && !isAttachmentDownload(req) {
		handleSignIn(w, req, sess, user)
	}

	log.Trace("SSPI Authorization: Logged in user %-v", user)
	return user, nil
}

// getConfig retrieves the SSPI configuration from login sources
func (s *SSPI) getConfig(ctx context.Context) (*sspi.Source, error) {
	sources, err := db.Find[auth.Source](ctx, auth.FindSourcesOptions{
		IsActive:  optional.Some(true),
		LoginType: auth.SSPI,
	})
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, errors.New("no active login sources of type SSPI found")
	}
	if len(sources) > 1 {
		return nil, errors.New("more than one active login source of type SSPI found")
	}
	return sources[0].Cfg.(*sspi.Source), nil
}

func (s *SSPI) shouldAuthenticate(req *http.Request) (shouldAuth bool) {
	shouldAuth = false
	path := strings.TrimSuffix(req.URL.Path, "/")
	if path == "/user/login" {
		if req.FormValue("user_name") != "" && req.FormValue("password") != "" {
			shouldAuth = false
		} else if req.FormValue("auth_with_sspi") == "1" {
			shouldAuth = true
		}
	} else if middleware.IsAPIPath(req) || isAttachmentDownload(req) {
		shouldAuth = true
	}
	return shouldAuth
}

// newUser creates a new user object for the purpose of automatic registration
// and populates its name and email with the information present in request headers.
func (s *SSPI) newUser(ctx context.Context, username string, cfg *sspi.Source) (*user_model.User, error) {
	email := gouuid.New().String() + "@localhost.localdomain"
	user := &user_model.User{
		Name:     username,
		Email:    email,
		Language: cfg.DefaultLanguage,
	}
	emailNotificationPreference := user_model.EmailNotificationsDisabled
	overwriteDefault := &user_model.CreateUserOverwriteOptions{
		IsActive:                     optional.Some(cfg.AutoActivateUsers),
		KeepEmailPrivate:             optional.Some(true),
		EmailNotificationsPreference: &emailNotificationPreference,
	}
	if err := user_model.CreateUser(ctx, user, &user_model.Meta{}, overwriteDefault); err != nil {
		return nil, err
	}

	return user, nil
}

// stripDomainNames removes NETBIOS domain name and separator from down-level logon names
// (eg. "DOMAIN\user" becomes "user"), and removes the UPN suffix (domain name) and separator
// from UPNs (eg. "user@domain.local" becomes "user")
func stripDomainNames(username string) string {
	if strings.Contains(username, "\\") {
		parts := strings.SplitN(username, "\\", 2)
		if len(parts) > 1 {
			username = parts[1]
		}
	} else if strings.Contains(username, "@") {
		parts := strings.Split(username, "@")
		if len(parts) > 1 {
			username = parts[0]
		}
	}
	return username
}

func replaceSeparators(username string, cfg *sspi.Source) string {
	newSep := cfg.SeparatorReplacement
	username = strings.ReplaceAll(username, "\\", newSep)
	username = strings.ReplaceAll(username, "/", newSep)
	username = strings.ReplaceAll(username, "@", newSep)
	return username
}

func sanitizeUsername(username string, cfg *sspi.Source) string {
	if len(username) == 0 {
		return ""
	}
	if cfg.StripDomainNames {
		username = stripDomainNames(username)
	}
	// Replace separators even if we have already stripped the domain name part,
	// as the username can contain several separators: eg. "MICROSOFT\useremail@live.com"
	username = replaceSeparators(username, cfg)
	return username
}
