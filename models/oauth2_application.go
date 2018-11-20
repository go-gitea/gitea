package models

import (
	gouuid "github.com/satori/go.uuid"
	"net/url"

	"code.gitea.io/gitea/modules/util"
	"github.com/Unknwon/com"

	"golang.org/x/crypto/bcrypt"
)

type OAuth2Application struct {
	ID   int64 `xorm:"pk autoincr"`
	UID  int64 `xorm:"INDEX"`
	User *User `xorm:"-"`

	Name string

	ClientID     string `xorm:"INDEX unique"`
	ClientSecret string

	RedirectURIs []string `xorm:"JSON TEXT"`

	CreatedUnix util.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix util.TimeStamp `xorm:"INDEX updated"`
}

func (app *OAuth2Application) TableName() string {
	return "oauth2_application"
}

func (app *OAuth2Application) LoadUser() (err error) {
	app.User = new(User)
	app.User, err = GetUserByID(app.UID)
	return
}

func (app *OAuth2Application) ContainsRedirectURI(redirectURI string) bool {
	return com.IsSliceContainsStr(app.RedirectURIs, redirectURI)
}

func (app *OAuth2Application) GenerateClientSecret() (string, error) {
	secret := gouuid.NewV4().String()
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	app.ClientSecret = string(hashedSecret)
	if _, err := x.ID(app.ID).Cols("client_secret").Update(app); err != nil {
		return "", err
	}
	return secret, nil
}

func (app *OAuth2Application) ValidateClientSecret(secret []byte) bool {
	return bcrypt.CompareHashAndPassword([]byte(app.ClientSecret), secret) == nil
}

// GetGrantByUserID returns a OAuth2Grant by its user and application ID
func (app *OAuth2Application) GetGrantByUserID(userID int64) (*OAuth2Grant, error) {
	return app.getGrantByUserID(x, userID)
}

func (app *OAuth2Application) getGrantByUserID(e Engine, userID int64) (grant *OAuth2Grant, err error) {
	grant = new(OAuth2Grant)
	if has, err := e.Where("user_id = ? AND application_id", userID, app.ID).Get(grant); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return grant, nil
}

// CreateGrant generates a grant for an user
func (app *OAuth2Application) CreateGrant(userID int64) (*OAuth2Grant, error) {
	return app.createGrant(x, userID)
}

func (app *OAuth2Application) createGrant(e Engine, userID int64) (*OAuth2Grant, error) {
	grant := &OAuth2Grant{
		ApplicationID: app.ID,
		UserID:        userID,
	}
	_, err := e.Insert(grant)
	if err != nil {
		return nil, err
	}
	return grant, nil
}

func GetOAuth2ApplicationByClientID(clientID string) (app *OAuth2Application, err error) {
	return getOAuth2ApplicationByClientID(x, clientID)
}

func getOAuth2ApplicationByClientID(e Engine, clientID string) (app *OAuth2Application, err error) {
	app = new(OAuth2Application)
	has, err := e.Where("client_id = ?", clientID).Get(app)
	if !has {
		return app, ErrOauthClientIDInvalid{ClientID: clientID}
	}
	return
}

// CreateOAuth2Application inserts a new oauth2 application
func CreateOAuth2Application(name string, userID int64) (*OAuth2Application, error) {
	return createOAuth2Application(x, name, userID)
}

func createOAuth2Application(e Engine, name string, userID int64) (*OAuth2Application, error) {
	secret := gouuid.NewV4().String()
	app := &OAuth2Application{
		UID:          userID,
		Name:         name,
		ClientID:     secret,
		RedirectURIs: []string{"http://localhost:3000"},
	}
	if _, err := e.Insert(app); err != nil {
		return nil, err
	}
	return app, nil
}

//////////////////////////////////////////////////////

type OAuth2AuthorizationCode struct {
	ID          int64        `xorm:"pk autoincr"`
	Grant       *OAuth2Grant `xorm:"-"`
	GrantID     int64
	Code        string `xorm:"INDEX unique"`
	RedirectURI string
	Lifetime    util.TimeStamp
}

func (code *OAuth2AuthorizationCode) TableName() string {
	return "oauth2_authorization_code"
}

func (code *OAuth2AuthorizationCode) GenerateRedirectURI(state string) (redirect *url.URL, err error) {
	if redirect, err = url.Parse(code.RedirectURI); err != nil {
		return
	}
	q := redirect.Query()
	if state != "" {
		q.Set("state", state)
	}
	q.Set("code", code.Code)
	redirect.RawQuery = q.Encode()
	return
}

//////////////////////////////////////////////////////

type OAuth2Grant struct {
	ID            int64          `xorm:"pk autoincr"`
	UserID        int64          `xorm:"INDEX unique(user_application)"`
	ApplicationID int64          `xorm:"INDEX unique(user_application)"`
	CreatedUnix   util.TimeStamp `xorm:"created"`
	UpdatedUnix   util.TimeStamp `xorm:"updated"`
}

func (grant *OAuth2Grant) TableName() string {
	return "oauth2_grant"
}

func (grant *OAuth2Grant) GenerateNewAuthorizationCode(redirectURI string) (*OAuth2AuthorizationCode, error) {
	return grant.generateNewAuthorizationCode(x, redirectURI)
}

func (grant *OAuth2Grant) generateNewAuthorizationCode(e Engine, redirectURI string) (*OAuth2AuthorizationCode, error) {
	secret := gouuid.NewV4().String()
	code := &OAuth2AuthorizationCode{
		Grant:       grant,
		GrantID:     grant.ID,
		RedirectURI: redirectURI,
		Code:        secret,
	}
	if _, err := e.Insert(code); err != nil {
		return nil, err
	}
	return code, nil
}
