package openidConnect

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/markbates/goth"
	"golang.org/x/oauth2"
)

const (
	// Standard Claims http://openid.net/specs/openid-connect-core-1_0.html#StandardClaims
	// fixed, cannot be changed
	subjectClaim  = "sub"
	expiryClaim   = "exp"
	audienceClaim = "aud"
	issuerClaim   = "iss"

	PreferredUsernameClaim = "preferred_username"
	EmailClaim             = "email"
	NameClaim              = "name"
	NicknameClaim          = "nickname"
	PictureClaim           = "picture"
	GivenNameClaim         = "given_name"
	FamilyNameClaim        = "family_name"
	AddressClaim           = "address"

	// Unused but available to set in Provider claims
	MiddleNameClaim          = "middle_name"
	ProfileClaim             = "profile"
	WebsiteClaim             = "website"
	EmailVerifiedClaim       = "email_verified"
	GenderClaim              = "gender"
	BirthdateClaim           = "birthdate"
	ZoneinfoClaim            = "zoneinfo"
	LocaleClaim              = "locale"
	PhoneNumberClaim         = "phone_number"
	PhoneNumberVerifiedClaim = "phone_number_verified"
	UpdatedAtClaim           = "updated_at"

	clockSkew = 10 * time.Second
)

// Provider is the implementation of `goth.Provider` for accessing OpenID Connect provider
type Provider struct {
	ClientKey    string
	Secret       string
	CallbackURL  string
	HTTPClient   *http.Client
	OpenIDConfig *OpenIDConfig
	config       *oauth2.Config
	providerName string

	UserIdClaims    []string
	NameClaims      []string
	NickNameClaims  []string
	EmailClaims     []string
	AvatarURLClaims []string
	FirstNameClaims []string
	LastNameClaims  []string
	LocationClaims  []string

	SkipUserInfoRequest bool
}

type OpenIDConfig struct {
	AuthEndpoint     string `json:"authorization_endpoint"`
	TokenEndpoint    string `json:"token_endpoint"`
	UserInfoEndpoint string `json:"userinfo_endpoint"`

	// If OpenID discovery is enabled, the end_session_endpoint field can optionally be provided
	// in the discovery endpoint response according to OpenID spec. See:
	// https://openid.net/specs/openid-connect-session-1_0-17.html#OPMetadata
	EndSessionEndpoint string `json:"end_session_endpoint, omitempty"`
	Issuer             string `json:"issuer"`
}

type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`

	// The OpenID spec defines the ID token as an optional response field in the
	// refresh token flow. As a result, a new ID token may not be returned in a successful
	// response.
	// See more: https://openid.net/specs/openid-connect-core-1_0.html#RefreshingAccessToken
	IdToken string `json:"id_token, omitempty"`

	// The OAuth spec defines the refresh token as an optional response field in the
	// refresh token flow. As a result, a new refresh token may not be returned in a successful
	// response.
	//See more: https://www.oauth.com/oauth2-servers/making-authenticated-requests/refreshing-an-access-token/
	RefreshToken string `json:"refresh_token,omitempty"`
}

// New creates a new OpenID Connect provider, and sets up important connection details.
// You should always call `openidConnect.New` to get a new Provider. Never try to create
// one manually.
// New returns an implementation of an OpenID Connect Authorization Code Flow
// See http://openid.net/specs/openid-connect-core-1_0.html#CodeFlowAuth
// ID Token decryption is not (yet) supported
// UserInfo decryption is not (yet) supported
func New(clientKey, secret, callbackURL, openIDAutoDiscoveryURL string, scopes ...string) (*Provider, error) {
	p := &Provider{
		ClientKey:   clientKey,
		Secret:      secret,
		CallbackURL: callbackURL,

		UserIdClaims:    []string{subjectClaim},
		NameClaims:      []string{NameClaim},
		NickNameClaims:  []string{NicknameClaim, PreferredUsernameClaim},
		EmailClaims:     []string{EmailClaim},
		AvatarURLClaims: []string{PictureClaim},
		FirstNameClaims: []string{GivenNameClaim},
		LastNameClaims:  []string{FamilyNameClaim},
		LocationClaims:  []string{AddressClaim},

		providerName: "openid-connect",
	}

	openIDConfig, err := getOpenIDConfig(p, openIDAutoDiscoveryURL)
	if err != nil {
		return nil, err
	}
	p.OpenIDConfig = openIDConfig

	p.config = newConfig(p, scopes, openIDConfig)
	return p, nil
}

// Name is the name used to retrieve this provider later.
func (p *Provider) Name() string {
	return p.providerName
}

// SetName is to update the name of the provider (needed in case of multiple providers of 1 type)
func (p *Provider) SetName(name string) {
	p.providerName = name
}

func (p *Provider) Client() *http.Client {
	return goth.HTTPClientWithFallBack(p.HTTPClient)
}

// Debug is a no-op for the openidConnect package.
func (p *Provider) Debug(debug bool) {}

// BeginAuth asks the OpenID Connect provider for an authentication end-point.
func (p *Provider) BeginAuth(state string) (goth.Session, error) {
	url := p.config.AuthCodeURL(state)
	session := &Session{
		AuthURL: url,
	}
	return session, nil
}

// FetchUser will use the the id_token and access requested information about the user.
func (p *Provider) FetchUser(session goth.Session) (goth.User, error) {
	sess := session.(*Session)

	expiresAt := sess.ExpiresAt

	if sess.IDToken == "" {
		return goth.User{}, fmt.Errorf("%s cannot get user information without id_token", p.providerName)
	}

	// decode returned id token to get expiry
	claims, err := decodeJWT(sess.IDToken)

	if err != nil {
		return goth.User{}, fmt.Errorf("oauth2: error decoding JWT token: %v", err)
	}

	expiry, err := p.validateClaims(claims)
	if err != nil {
		return goth.User{}, fmt.Errorf("oauth2: error validating JWT token: %v", err)
	}

	if expiry.Before(expiresAt) {
		expiresAt = expiry
	}

	if err := p.getUserInfo(sess.AccessToken, claims); err != nil {
		return goth.User{}, err
	}

	user := goth.User{
		AccessToken:  sess.AccessToken,
		Provider:     p.Name(),
		RefreshToken: sess.RefreshToken,
		ExpiresAt:    expiresAt,
		RawData:      claims,
		IDToken:      sess.IDToken,
	}

	p.userFromClaims(claims, &user)
	return user, err
}

//RefreshTokenAvailable refresh token is provided by auth provider or not
func (p *Provider) RefreshTokenAvailable() bool {
	return true
}

//RefreshToken get new access token based on the refresh token
func (p *Provider) RefreshToken(refreshToken string) (*oauth2.Token, error) {
	token := &oauth2.Token{RefreshToken: refreshToken}
	ts := p.config.TokenSource(oauth2.NoContext, token)
	newToken, err := ts.Token()
	if err != nil {
		return nil, err
	}
	return newToken, err
}

// The ID token is a fundamental part of the OpenID connect refresh token flow but is not part of the OAuth flow.
// The existing RefreshToken function leverages the OAuth library's refresh token mechanism, ignoring the refreshed
// ID token. As a result, a new function needs to be exposed (rather than changing the existing function, for backwards
// compatibility purposes) that also returns the id_token in the OpenID refresh token flow API response
// Learn more about ID tokens: https://openid.net/specs/openid-connect-core-1_0.html#IDToken
func (p *Provider) RefreshTokenWithIDToken(refreshToken string) (*RefreshTokenResponse, error) {
	urlValues := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {p.ClientKey},
		"client_secret": {p.Secret},
	}
	req, err := http.NewRequest("POST", p.OpenIDConfig.TokenEndpoint, strings.NewReader(urlValues.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.Client().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Non-200 response from RefreshToken: %d, WWW-Authenticate=%s", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	refreshTokenResponse := &RefreshTokenResponse{}

	err = json.Unmarshal(body, refreshTokenResponse)
	if err != nil {
		return nil, err
	}

	return refreshTokenResponse, nil
}

// validate according to standard, returns expiry
// http://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation
func (p *Provider) validateClaims(claims map[string]interface{}) (time.Time, error) {
	audience := getClaimValue(claims, []string{audienceClaim})
	if audience != p.ClientKey {
		found := false
		audiences := getClaimValues(claims, []string{audienceClaim})
		for _, aud := range audiences {
			if aud == p.ClientKey {
				found = true
				break
			}
		}
		if !found {
			return time.Time{}, errors.New("audience in token does not match client key")
		}
	}

	issuer := getClaimValue(claims, []string{issuerClaim})
	if issuer != p.OpenIDConfig.Issuer {
		return time.Time{}, errors.New("issuer in token does not match issuer in OpenIDConfig discovery")
	}

	// expiry is required for JWT, not for UserInfoResponse
	// is actually a int64, so force it in to that type
	expiryClaim := int64(claims[expiryClaim].(float64))
	expiry := time.Unix(expiryClaim, 0)
	if expiry.Add(clockSkew).Before(time.Now()) {
		return time.Time{}, errors.New("user info JWT token is expired")
	}
	return expiry, nil
}

func (p *Provider) userFromClaims(claims map[string]interface{}, user *goth.User) {
	// required
	user.UserID = getClaimValue(claims, p.UserIdClaims)

	user.Name = getClaimValue(claims, p.NameClaims)
	user.NickName = getClaimValue(claims, p.NickNameClaims)
	user.Email = getClaimValue(claims, p.EmailClaims)
	user.AvatarURL = getClaimValue(claims, p.AvatarURLClaims)
	user.FirstName = getClaimValue(claims, p.FirstNameClaims)
	user.LastName = getClaimValue(claims, p.LastNameClaims)
	user.Location = getClaimValue(claims, p.LocationClaims)
}

func (p *Provider) getUserInfo(accessToken string, claims map[string]interface{}) error {
	// skip if there is no UserInfoEndpoint or is explicitly disabled
	if p.OpenIDConfig.UserInfoEndpoint == "" || p.SkipUserInfoRequest {
		return nil
	}

	userInfoClaims, err := p.fetchUserInfo(p.OpenIDConfig.UserInfoEndpoint, accessToken)
	if err != nil {
		return err
	}

	// The sub (subject) Claim MUST always be returned in the UserInfo Response.
	// http://openid.net/specs/openid-connect-core-1_0.html#UserInfoResponse
	userInfoSubject := getClaimValue(userInfoClaims, []string{subjectClaim})
	if userInfoSubject == "" {
		return fmt.Errorf("userinfo response did not contain a 'sub' claim: %#v", userInfoClaims)
	}

	// The sub Claim in the UserInfo Response MUST be verified to exactly match the sub Claim in the ID Token;
	// if they do not match, the UserInfo Response values MUST NOT be used.
	// http://openid.net/specs/openid-connect-core-1_0.html#UserInfoResponse
	subject := getClaimValue(claims, []string{subjectClaim})
	if userInfoSubject != subject {
		return fmt.Errorf("userinfo 'sub' claim (%s) did not match id_token 'sub' claim (%s)", userInfoSubject, subject)
	}

	// Merge in userinfo claims in case id_token claims contained some that userinfo did not
	for k, v := range userInfoClaims {
		claims[k] = v
	}

	return nil
}

// fetch and decode JSON from the given UserInfo URL
func (p *Provider) fetchUserInfo(url, accessToken string) (map[string]interface{}, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := p.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Non-200 response from UserInfo: %d, WWW-Authenticate=%s", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}

	// The UserInfo Claims MUST be returned as the members of a JSON object
	// http://openid.net/specs/openid-connect-core-1_0.html#UserInfoResponse
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return unMarshal(data)
}

func getOpenIDConfig(p *Provider, openIDAutoDiscoveryURL string) (*OpenIDConfig, error) {
	res, err := p.Client().Get(openIDAutoDiscoveryURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	openIDConfig := &OpenIDConfig{}
	err = json.Unmarshal(body, openIDConfig)
	if err != nil {
		return nil, err
	}

	return openIDConfig, nil
}

func newConfig(provider *Provider, scopes []string, openIDConfig *OpenIDConfig) *oauth2.Config {
	c := &oauth2.Config{
		ClientID:     provider.ClientKey,
		ClientSecret: provider.Secret,
		RedirectURL:  provider.CallbackURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  openIDConfig.AuthEndpoint,
			TokenURL: openIDConfig.TokenEndpoint,
		},
		Scopes: []string{},
	}

	if len(scopes) > 0 {
		foundOpenIDScope := false

		for _, scope := range scopes {
			if scope == "openid" {
				foundOpenIDScope = true
			}
			c.Scopes = append(c.Scopes, scope)
		}

		if !foundOpenIDScope {
			c.Scopes = append(c.Scopes, "openid")
		}
	} else {
		c.Scopes = []string{"openid"}
	}

	return c
}

func getClaimValue(data map[string]interface{}, claims []string) string {
	for _, claim := range claims {
		if value, ok := data[claim]; ok {
			if stringValue, ok := value.(string); ok && len(stringValue) > 0 {
				return stringValue
			}
		}
	}

	return ""
}

func getClaimValues(data map[string]interface{}, claims []string) []string {
	var result []string

	for _, claim := range claims {
		if value, ok := data[claim]; ok {
			if stringValues, ok := value.([]interface{}); ok {
				for _, stringValue := range stringValues {
					if s, ok := stringValue.(string); ok && len(s) > 0 {
						result = append(result, s)
					}
				}
			}
		}
	}

	return result
}

// decodeJWT decodes a JSON Web Token into a simple map
// http://openid.net/specs/draft-jones-json-web-token-07.html
func decodeJWT(jwt string) (map[string]interface{}, error) {
	jwtParts := strings.Split(jwt, ".")
	if len(jwtParts) != 3 {
		return nil, errors.New("jws: invalid token received, not all parts available")
	}

	decodedPayload, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(jwtParts[1])

	if err != nil {
		return nil, err
	}

	return unMarshal(decodedPayload)
}

func unMarshal(payload []byte) (map[string]interface{}, error) {
	data := make(map[string]interface{})

	return data, json.NewDecoder(bytes.NewBuffer(payload)).Decode(&data)
}
