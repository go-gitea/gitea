// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/avatars"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/auth/source/sspi"

	gouuid "github.com/google/uuid"
	"github.com/quasoft/websspi"
)

var (
	// sspiAuth is a global instance of the websspi authentication package,
	// which is used to avoid acquiring the server credential handle on
	// every request
	sspiAuth     *websspi.Authenticator
	sspiAuthOnce sync.Once

	// Ensure the struct implements the interface.
	_ Method = &SSPI{}
)

// SSPI implements the SingleSignOn interface and authenticates requests
// via the built-in SSPI module in Windows for SPNEGO authentication.
// On successful authentication returns a valid user object.
// Returns nil if authentication fails.
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
	var errInit error
	sspiAuthOnce.Do(func() {
		config := websspi.NewConfig()
		sspiAuth, errInit = websspi.New(config)
	})
	if errInit != nil {
		return nil, errInit
	}

	if !s.shouldAuthenticate(req) {
		return nil, nil
	}

	cfg, err := s.getConfig()
	if err != nil {
		log.Error("could not get SSPI config: %v", err)
		return nil, err
	}

	log.Trace("SSPI Authorization: Attempting to authenticate")
	userInfo, outToken, err := sspiAuth.Authenticate(req, w)
	if err != nil {
		sspiAuth.AppendAuthenticateHeader(w, outToken)
		// The SSPI workflow requires a 401 StatusUnauthorized response code
		// which gets set by the auth routes.
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
		user, err = s.newUser(username, cfg)
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
func (s *SSPI) getConfig() (*sspi.Source, error) {
	sources, err := auth.ActiveSources(auth.SSPI)
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
	if middleware.IsLoginPath(req) {
		if req.FormValue("user_name") != "" && req.FormValue("password") != "" {
			return false
		}
		b, _ := strconv.ParseBool(req.FormValue("auth_with_sspi"))
		return b
	}
	return middleware.IsAPIPath(req) || isAttachmentDownload(req)
}

// newUser creates a new user object for the purpose of automatic registration
// and populates its name and email with the information present in request headers.
func (s *SSPI) newUser(username string, cfg *sspi.Source) (*user_model.User, error) {
	email := gouuid.New().String() + "@localhost.localdomain"
	user := &user_model.User{
		Name:            username,
		Email:           email,
		Passwd:          gouuid.New().String(),
		Language:        cfg.DefaultLanguage,
		UseCustomAvatar: true,
		Avatar:          avatars.DefaultAvatarLink(),
	}
	overwriteDefault := &user_model.CreateUserOverwriteOptions{
		IsActive:                     util.OptionalBoolOf(cfg.AutoActivateUsers),
		KeepEmailPrivate:             util.OptionalBoolTrue,
		EmailNotificationsPreference: util.ToPointer(user_model.EmailNotificationsDisabled),
	}
	if err := user_model.CreateUser(user, overwriteDefault); err != nil {
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
