package models

import (
	gouuid "github.com/satori/go.uuid"

	"code.gitea.io/gitea/modules/util"
	"fmt"
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

func (app *OAuth2Application) LoadUser() (err error) {
	app.User = new(User)
	app.User, err = GetUserByID(app.UID)
	return
}

func (app *OAuth2Application) ContainsRedirectURI(redirectURI string) bool {
	return com.IsSliceContainsStr(app.RedirectURIs, redirectURI)
}

func (app *OAuth2Application) GenerateClientSecret() ([]byte, error) {
	secret := gouuid.NewV4().Bytes()
	hashedSecret, err := bcrypt.GenerateFromPassword(secret, bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	app.ClientSecret = string(hashedSecret)
	return secret, nil
}

func (app *OAuth2Application) ValidateClientSecret(secret []byte) bool {
	return bcrypt.CompareHashAndPassword([]byte(app.ClientSecret), secret) == nil
}

func GetOAuth2ApplicationByClientID(clientID string) (app *OAuth2Application, err error) {
	return getOAuth2ApplicationByClientID(x, clientID)
}

func getOAuth2ApplicationByClientID(e Engine, clientID string) (app *OAuth2Application, err error) {
	app = new(OAuth2Application)
	_, err = e.Where("client_id = ?", clientID).Get(app)
	return
}

type AuthorizeErrorCode string

const (
	ErrorCodeInvalidRequest          AuthorizeErrorCode = "invalid_request"
	ErrorCodeUnauthorizedClient      AuthorizeErrorCode = "unauthorized_client"
	ErrorCodeAccessDenied            AuthorizeErrorCode = "access_denied"
	ErrorCodeUnsupportedResponseType AuthorizeErrorCode = "unsupported_response_type"
	ErrorCodeInvalidScope            AuthorizeErrorCode = "invalid_scope"
	ErrorCodeServerError             AuthorizeErrorCode = "server_error"
	ErrorCodeTemporaryUnavailable                       = "temporarily_unavailable"
)

type AuthorizeError struct {
	ErrorCode        AuthorizeErrorCode `json:"error" form:"error"`
	ErrorDescription string
	State            string
}

func (err AuthorizeError) Error() string {
	return fmt.Sprintf("%s: %s", err.ErrorCode, err.ErrorDescription)
}
