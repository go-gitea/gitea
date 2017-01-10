package oauth2

import (
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/gorilla/sessions"
	"os"
	"github.com/markbates/goth/providers/github"
	"code.gitea.io/gitea/modules/setting"
	"net/http"
	"fmt"
	"code.gitea.io/gitea/modules/context"
	"github.com/go-macaron/session"
	"github.com/satori/go.uuid"
	"encoding/base64"
	"strings"
)

var (
	oAuthUserSessionKey = "oauth2-user"
	oAuthStateUniqueIdSessionKey = "oauth2-state-key"
)

func init() {
	gothic.Store = sessions.NewFilesystemStore(os.TempDir(), []byte("gitea-oauth-sessions"))
}

// Auth OAuth2 auth service
func Auth(provider, clientId, clientSecret, userName, passwd string) (string, error) {
	goth.UseProviders(
		github.New(clientId, clientSecret, setting.AppURL + "user/oauth2/github/callback"),
	)

	ctx := context.Context{}
	request := ctx.Req.Request
	session := ctx.Session

	oauth2User := session.Get(oAuthUserSessionKey)
	if oauth2User != nil {
		// already authenticated, stop OAuth2 flow
		session.Delete(oAuthUserSessionKey)
		return oauth2User.(goth.User).Email, nil
	}

	gothic.SetState = func(req *http.Request) string {
		state, uniqueId := encodeState(req.URL.String())

		session.Set(oAuthStateUniqueIdSessionKey, uniqueId)

		return state
	}
	gothic.GetProviderName = func(r *http.Request) (string, error) {
		provider := ctx.Params(":provider")
		if provider == "" {
			provider = "github"
		}
		return provider, nil
	}

	gothic.BeginAuthHandler(ctx.Resp, request)


	return nil, nil
}

// handle OAuth callback, resolve to a goth user and send back to original url
// this will trigger a new authentication request, but because we save it in the session we can use that
func Callback(ctx *context.Context, session session.Store) {
	request := ctx.Req.Request

	res := ctx.Resp

	user, err := gothic.CompleteUserAuth(res, request)
	if err != nil {
		fmt.Fprintln(res, err)
		return
	}

	redirectUrl := ""

	state := gothic.GetState(request)
	if state == "" {
		redirectUrl = "/TROUBLE"
	}

	decodedState, uniqueId, err := decodeState(state)
	if err != nil {
		redirectUrl = "/TROUBLE2"
	} else {
		uniqueIdSession := session.Get(oAuthStateUniqueIdSessionKey)
		session.Delete(oAuthStateUniqueIdSessionKey)

		if uniqueIdSession != nil && uniqueIdSession == uniqueId {
			redirectUrl = decodedState
			session.Set(oAuthUserSessionKey, user)
		} else {
			redirectUrl = "/TROUBLE3"
		}
	}

	ctx.Redirect(redirectUrl)
}

func encodeState(state string) (string, string) {
	uniqueId := uuid.NewV4().String()
	data := []byte(uniqueId + "|" + state)

	return base64.URLEncoding.EncodeToString(data), uniqueId
}

func decodeState(encodedState string) (string, string, error) {
	data, err := base64.URLEncoding.DecodeString(encodedState)

	if err != nil {
		return nil, nil, err
	}

	decodedState := string(data[:])
	slices := strings.Split(decodedState, "|")

	uniqueId := slices[0]

	// validate uuid
	_, err = uuid.FromString(uniqueId)
	if err != nil {
		return nil, nil, err
	}

	state := slices[1]

	return state, uniqueId, nil
}
