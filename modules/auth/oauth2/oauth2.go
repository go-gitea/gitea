package oauth2

import (
	"code.gitea.io/gitea/modules/setting"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
	"gopkg.in/macaron.v1"
	"net/http"
	"os"
	"github.com/go-macaron/session"
	"github.com/satori/go.uuid"
	"encoding/base64"
	"strings"
)

var (
	sessionUsersStoreKey         = "gitea-oauth-sessions"
	oAuthUserSessionKey          = "oauth2-user"
	oAuthStateUniqueIDSessionKey = "oauth2-state-key"
)

func init() {
	gothic.Store = sessions.NewFilesystemStore(os.TempDir(), []byte(sessionUsersStoreKey))
}

// Auth OAuth2 auth service
func Auth(provider, clientID, clientSecret string, ctx *macaron.Context, session session.Store) {
	callbackURL := setting.AppURL + "user/oauth2/" + provider + "/callback"

	goth.UseProviders(
		github.New(clientID, clientSecret, callbackURL, "user:email"),
	)

	gothic.GetProviderName = func(req *http.Request) (string, error) {
		return provider, nil
	}

	gothic.SetState = func(req *http.Request) string {
		state, uniqueID := encodeState(req.URL.String())

		session.Set(oAuthStateUniqueIDSessionKey, uniqueID)

		return state
	}

	request := ctx.Req.Request
	response := ctx.Resp

	/*
	oauth2User := session.Get(oAuthUserSessionKey)
	session.Delete(oAuthUserSessionKey)
	if oauth2User != nil {
		// already authenticated, stop OAuth2 authentication flow
		http.Redirect(response, request, callbackUrl, http.StatusTemporaryRedirect)
		return
	}
	*/

	gothic.BeginAuthHandler(response, request)
}

// ProviderCallback handles OAuth callback, resolve to a goth user and send back to original url
// this will trigger a new authentication request, but because we save it in the session we can use that
func ProviderCallback(provider string, ctx *macaron.Context, session session.Store) (goth.User, string, error) {
	request := ctx.Req.Request

	res := ctx.Resp

	gothic.GetProviderName = func(req *http.Request) (string, error) {
		return provider, nil
	}

	user, err := gothic.CompleteUserAuth(res, request)
	if err != nil {
		return user, "", err
	}

	redirectURL := ""

	state := gothic.GetState(request)
	if state == "" {
		redirectURL = "TROUBLE"
	}

	if _, uniqueID, err := decodeState(state); err != nil {
		redirectURL = "TROUBLE2"
	} else {
		uniqueIDSession := session.Get(oAuthStateUniqueIDSessionKey)
		session.Delete(oAuthStateUniqueIDSessionKey)

		if uniqueIDSession != nil && uniqueIDSession == uniqueID {
			redirectURL = ""
			session.Set(oAuthUserSessionKey, user)
		} else {
			redirectURL = "TROUBLE3"
		}
	}

	return user, redirectURL, nil
}

func encodeState(state string) (string, string) {
	uniqueID := uuid.NewV4().String()
	data := []byte(uniqueID + "|" + state)

	return base64.URLEncoding.EncodeToString(data), uniqueID
}

func decodeState(encodedState string) (string, string, error) {
	data, err := base64.URLEncoding.DecodeString(encodedState)

	if err != nil {
		return "", "", err
	}

	decodedState := string(data[:])
	slices := strings.Split(decodedState, "|")

	uniqueID := slices[0]

	// validate uuid
	_, err = uuid.FromString(uniqueID)
	if err != nil {
		return "", "", err
	}

	state := slices[1]

	return state, uniqueID, nil
}
