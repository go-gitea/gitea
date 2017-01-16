package oauth2

import (
	"code.gitea.io/gitea/modules/setting"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
	"net/http"
	"os"
	"github.com/satori/go.uuid"
)

var (
	sessionUsersStoreKey = "gitea-oauth-sessions"
)

func init() {
	gothic.Store = sessions.NewFilesystemStore(os.TempDir(), []byte(sessionUsersStoreKey))
}

// Auth OAuth2 auth service
func Auth(provider, clientID, clientSecret string, request *http.Request, response http.ResponseWriter) {
	callbackURL := setting.AppURL + "user/oauth2/" + provider + "/callback"

	goth.UseProviders(
		github.New(clientID, clientSecret, callbackURL, "user:email"),
	)

	gothic.GetProviderName = func(req *http.Request) (string, error) {
		return provider, nil
	}

	gothic.SetState = func(req *http.Request) string {
		return uuid.NewV4().String()
	}

	gothic.BeginAuthHandler(response, request)
}

// ProviderCallback handles OAuth callback, resolve to a goth user and send back to original url
// this will trigger a new authentication request, but because we save it in the session we can use that
func ProviderCallback(provider string, request *http.Request, response http.ResponseWriter) (goth.User, string, error) {
	gothic.GetProviderName = func(req *http.Request) (string, error) {
		return provider, nil
	}

	user, err := gothic.CompleteUserAuth(response, request)
	if err != nil {
		return user, "", err
	}

	return user, "", nil
}
