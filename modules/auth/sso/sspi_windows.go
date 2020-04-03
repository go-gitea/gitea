// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sso

import (
	"errors"
	"reflect"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"gitea.com/macaron/macaron"
	"gitea.com/macaron/session"

	"github.com/quasoft/websspi"
	gouuid "github.com/satori/go.uuid"
)

const (
	tplSignIn base.TplName = "user/auth/signin"
)

var (
	// sspiAuth is a global instance of the websspi authentication package,
	// which is used to avoid acquiring the server credential handle on
	// every request
	sspiAuth *websspi.Authenticator

	// Ensure the struct implements the interface.
	_ SingleSignOn = &SSPI{}
)

// SSPI implements the SingleSignOn interface and authenticates requests
// via the built-in SSPI module in Windows for SPNEGO authentication.
// On successful authentication returns a valid user object.
// Returns nil if authentication fails.
type SSPI struct {
}

// Init creates a new global websspi.Authenticator object
func (s *SSPI) Init() error {
	config := websspi.NewConfig()
	var err error
	sspiAuth, err = websspi.New(config)
	return err
}

// Free releases resources used by the global websspi.Authenticator object
func (s *SSPI) Free() error {
	return sspiAuth.Free()
}

// IsEnabled checks if there is an active SSPI authentication source
func (s *SSPI) IsEnabled() bool {
	return models.IsSSPIEnabled()
}

// VerifyAuthData uses SSPI (Windows implementation of SPNEGO) to authenticate the request.
// If authentication is successful, returs the corresponding user object.
// If negotiation should continue or authentication fails, immediately returns a 401 HTTP
// response code, as required by the SPNEGO protocol.
func (s *SSPI) VerifyAuthData(ctx *macaron.Context, sess session.Store) *models.User {
	if !s.shouldAuthenticate(ctx) {
		return nil
	}

	cfg, err := s.getConfig()
	if err != nil {
		log.Error("could not get SSPI config: %v", err)
		return nil
	}

	userInfo, outToken, err := sspiAuth.Authenticate(ctx.Req.Request, ctx.Resp)
	if err != nil {
		log.Warn("Authentication failed with error: %v\n", err)
		sspiAuth.AppendAuthenticateHeader(ctx.Resp, outToken)

		// Include the user login page in the 401 response to allow the user
		// to login with another authentication method if SSPI authentication
		// fails
		addFlashErr(ctx, ctx.Tr("auth.sspi_auth_failed"))
		ctx.Data["EnableOpenIDSignIn"] = setting.Service.EnableOpenIDSignIn
		ctx.Data["EnableSSPI"] = true
		ctx.HTML(401, string(tplSignIn))
		return nil
	}
	if outToken != "" {
		sspiAuth.AppendAuthenticateHeader(ctx.Resp, outToken)
	}

	username := sanitizeUsername(userInfo.Username, cfg)
	if len(username) == 0 {
		return nil
	}
	log.Info("Authenticated as %s\n", username)

	user, err := models.GetUserByName(username)
	if err != nil {
		if !models.IsErrUserNotExist(err) {
			log.Error("GetUserByName: %v", err)
			return nil
		}
		if !cfg.AutoCreateUsers {
			log.Error("User '%s' not found", username)
			return nil
		}
		user, err = s.newUser(ctx, username, cfg)
		if err != nil {
			log.Error("CreateUser: %v", err)
			return nil
		}
	}

	// Make sure requests to API paths and PWA resources do not create a new session
	if !isAPIPath(ctx) && !isAttachmentDownload(ctx) {
		handleSignIn(ctx, sess, user)
	}

	return user
}

// getConfig retrieves the SSPI configuration from login sources
func (s *SSPI) getConfig() (*models.SSPIConfig, error) {
	sources, err := models.ActiveLoginSources(models.LoginSSPI)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, errors.New("no active login sources of type SSPI found")
	}
	if len(sources) > 1 {
		return nil, errors.New("more than one active login source of type SSPI found")
	}
	return sources[0].SSPI(), nil
}

func (s *SSPI) shouldAuthenticate(ctx *macaron.Context) (shouldAuth bool) {
	shouldAuth = false
	path := strings.TrimSuffix(ctx.Req.URL.Path, "/")
	if path == "/user/login" {
		if ctx.Req.FormValue("user_name") != "" && ctx.Req.FormValue("password") != "" {
			shouldAuth = false
		} else if ctx.Req.FormValue("auth_with_sspi") == "1" {
			shouldAuth = true
		}
	} else if isAPIPath(ctx) || isAttachmentDownload(ctx) {
		shouldAuth = true
	}
	return
}

// newUser creates a new user object for the purpose of automatic registration
// and populates its name and email with the information present in request headers.
func (s *SSPI) newUser(ctx *macaron.Context, username string, cfg *models.SSPIConfig) (*models.User, error) {
	email := gouuid.NewV4().String() + "@localhost.localdomain"
	user := &models.User{
		Name:                         username,
		Email:                        email,
		KeepEmailPrivate:             true,
		Passwd:                       gouuid.NewV4().String(),
		IsActive:                     cfg.AutoActivateUsers,
		Language:                     cfg.DefaultLanguage,
		UseCustomAvatar:              true,
		Avatar:                       base.DefaultAvatarLink(),
		EmailNotificationsPreference: models.EmailNotificationsDisabled,
	}
	if err := models.CreateUser(user); err != nil {
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

func replaceSeparators(username string, cfg *models.SSPIConfig) string {
	newSep := cfg.SeparatorReplacement
	username = strings.ReplaceAll(username, "\\", newSep)
	username = strings.ReplaceAll(username, "/", newSep)
	username = strings.ReplaceAll(username, "@", newSep)
	return username
}

func sanitizeUsername(username string, cfg *models.SSPIConfig) string {
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

// addFlashErr adds an error message to the Flash object mapped to a macaron.Context
func addFlashErr(ctx *macaron.Context, err string) {
	fv := ctx.GetVal(reflect.TypeOf(&session.Flash{}))
	if !fv.IsValid() {
		return
	}
	flash, ok := fv.Interface().(*session.Flash)
	if !ok {
		return
	}
	flash.Error(err)
	ctx.Data["Flash"] = flash
}

// init registers the SSPI auth method as the last method in the list.
// The SSPI plugin is expected to be executed last, as it returns 401 status code if negotiation
// fails (or if negotiation should continue), which would prevent other authentication methods
// to execute at all.
func init() {
	Register(&SSPI{})
}
