package oauth2

import (
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/log"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
	"net/http"
	"os"
	"github.com/satori/go.uuid"
	"path/filepath"
)

var (
	sessionUsersStoreKey = "gitea-oauth2-sessions"
	providerHeaderKey    = "gitea-oauth2-provider"
)

func init() {
	dir, _ := setting.WorkDir()
	tmpDir := filepath.Join(dir, "data", "sessions", "oauth2")
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		log.Fatal(4, "Fail to create dir %s: %v", tmpDir, err)
	}

	gothic.Store = sessions.NewFilesystemStore(tmpDir, []byte(sessionUsersStoreKey))

	gothic.SetState = func(req *http.Request) string {
		return uuid.NewV4().String()
	}

	gothic.GetProviderName = func(req *http.Request) (string, error) {
		return req.Header.Get(providerHeaderKey), nil
	}
}

// Auth OAuth2 auth service
func Auth(provider, clientID, clientSecret string, request *http.Request, response http.ResponseWriter) {
	// not sure if goth is thread safe (?) when using multiple providers
	request.Header.Set(providerHeaderKey, provider)

	if gothProvider, _ := goth.GetProvider(provider); gothProvider == nil {
		goth.UseProviders(
			github.New(clientID, clientSecret, setting.AppURL+"user/oauth2/"+provider+"/callback", "user:email"),
		)
	}

	gothic.BeginAuthHandler(response, request)
}

// ProviderCallback handles OAuth callback, resolve to a goth user and send back to original url
// this will trigger a new authentication request, but because we save it in the session we can use that
func ProviderCallback(provider string, request *http.Request, response http.ResponseWriter) (goth.User, string, error) {
	// not sure if goth is thread safe (?) when using multiple providers
	request.Header.Set(providerHeaderKey, provider)

	user, err := gothic.CompleteUserAuth(response, request)
	if err != nil {
		return user, "", err
	}

	return user, "", nil
}
