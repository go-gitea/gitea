package sso

import (
	"errors"
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
	_ SingleSignOn = &Basic{}
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
	sources, err := models.ActiveLoginSources(models.LoginSSPI)
	if err != nil {
		log.Warn("Could not get login sources: %v\n", err)
		return false
	}
	return len(sources) > 0
}

// Priority determines the order in which authentication methods are executed.
// The lower the priority, the sooner the plugin is executed.
// The SSPI plugin should be executed last as it returns 401 status code
// if negotiation fails or should continue, which would prevent other
// authentication methods to execute at all.
func (s *SSPI) Priority() int {
	return 50000
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

	newSep := cfg.SeparatorReplacement
	username := strings.ReplaceAll(userInfo.Username, "\\", newSep)
	username = strings.ReplaceAll(username, "/", newSep)
	username = strings.ReplaceAll(username, "@", newSep)
	log.Info("Authenticated as %s\n", username)
	if len(username) == 0 {
		return nil
	}

	user, err := models.GetUserByName(username)
	if err != nil {
		if models.IsErrUserNotExist(err) && cfg.AutoCreateUsers {
			return s.newUser(ctx, username, cfg)
		}
		log.Error("GetUserByName: %v", err)
		return nil
	}

	// Make sure requests to API paths and PWA resources do not create a new session
	if !isAPIPath(ctx.Req.URL.Path) {
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
	return sources[0].SSPI(), nil
}

func (s *SSPI) shouldAuthenticate(ctx *macaron.Context) bool {
	path := strings.TrimSuffix(ctx.Req.URL.Path, "/")
	if path == "/user/login" && ctx.Req.FormValue("user_name") != "" && ctx.Req.FormValue("password") != "" {
		return false
	} else if ctx.Req.FormValue("auth_with_sspi") == "1" {
		return true
	}
	return !isPublicPage(ctx) && !isPublicResource(ctx)
}

// newUser creates a new user object for the purpose of automatic registration
// and populates its name and email with the information present in request headers.
func (s *SSPI) newUser(ctx *macaron.Context, username string, cfg *models.SSPIConfig) *models.User {
	email := gouuid.NewV4().String() + "@example.org"
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
		// FIXME: should I create a system notice?
		log.Error("CreateUser: %v", err)
		return nil
	}
	return user
}

// init registers the plugin to the list of available SSO methods
func init() {
	Register(&SSPI{})
}
