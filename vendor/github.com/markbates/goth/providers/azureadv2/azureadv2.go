package azureadv2

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/markbates/goth"
	"golang.org/x/oauth2"
)

// also https://docs.microsoft.com/en-us/azure/active-directory/develop/active-directory-v2-protocols#endpoints
const (
	authURLTemplate  string = "https://login.microsoftonline.com/%s/oauth2/v2.0/authorize"
	tokenURLTemplate string = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
	graphAPIResource string = "https://graph.microsoft.com/v1.0/"
)

type (
	// TenantType are the well known tenant types to scope the users that can authenticate. TenantType is not an
	// exclusive list of Azure Tenants which can be used. A consumer can also use their own Tenant ID to scope
	// authentication to their specific Tenant either through the Tenant ID or the friendly domain name.
	//
	// see also https://docs.microsoft.com/en-us/azure/active-directory/develop/active-directory-v2-protocols#endpoints
	TenantType string

	// Provider is the implementation of `goth.Provider` for accessing AzureAD V2.
	Provider struct {
		ClientKey    string
		Secret       string
		CallbackURL  string
		HTTPClient   *http.Client
		config       *oauth2.Config
		providerName string
	}

	// ProviderOptions are the collection of optional configuration to provide when constructing a Provider
	ProviderOptions struct {
		Scopes []ScopeType
		Tenant TenantType
	}
)

// These are the well known Azure AD Tenants. These are not an exclusive list of all Tenants
//
// See also https://docs.microsoft.com/en-us/azure/active-directory/develop/active-directory-v2-protocols#endpoints
const (
	// CommonTenant allows users with both personal Microsoft accounts and work/school accounts from Azure Active
	// Directory to sign into the application.
	CommonTenant TenantType = "common"

	// OrganizationsTenant allows only users with work/school accounts from Azure Active Directory to sign into the application.
	OrganizationsTenant TenantType = "organizations"

	// ConsumersTenant allows only users with personal Microsoft accounts (MSA) to sign into the application.
	ConsumersTenant TenantType = "consumers"
)

// New creates a new AzureAD provider, and sets up important connection details.
// You should always call `AzureAD.New` to get a new Provider. Never try to create
// one manually.
func New(clientKey, secret, callbackURL string, opts ProviderOptions) *Provider {
	p := &Provider{
		ClientKey:    clientKey,
		Secret:       secret,
		CallbackURL:  callbackURL,
		providerName: "azureadv2",
	}

	p.config = newConfig(p, opts)
	return p
}

func newConfig(provider *Provider, opts ProviderOptions) *oauth2.Config {
	tenant := opts.Tenant
	if tenant == "" {
		tenant = CommonTenant
	}

	c := &oauth2.Config{
		ClientID:     provider.ClientKey,
		ClientSecret: provider.Secret,
		RedirectURL:  provider.CallbackURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf(authURLTemplate, tenant),
			TokenURL: fmt.Sprintf(tokenURLTemplate, tenant),
		},
		Scopes: []string{},
	}

	if len(opts.Scopes) > 0 {
		c.Scopes = append(c.Scopes, scopesToStrings(opts.Scopes...)...)
	} else {
		defaultScopes := scopesToStrings(OpenIDScope, ProfileScope, EmailScope, UserReadScope)
		c.Scopes = append(c.Scopes, defaultScopes...)
	}

	return c
}

// Name is the name used to retrieve this provider later.
func (p *Provider) Name() string {
	return p.providerName
}

// SetName is to update the name of the provider (needed in case of multiple providers of 1 type)
func (p *Provider) SetName(name string) {
	p.providerName = name
}

// Client is HTTP client to be used in all fetch operations.
func (p *Provider) Client() *http.Client {
	return goth.HTTPClientWithFallBack(p.HTTPClient)
}

// Debug is a no-op for the package
func (p *Provider) Debug(debug bool) {}

// BeginAuth asks for an authentication end-point for AzureAD.
func (p *Provider) BeginAuth(state string) (goth.Session, error) {
	authURL := p.config.AuthCodeURL(state)

	return &Session{
		AuthURL: authURL,
	}, nil
}

// FetchUser will go to AzureAD and access basic information about the user.
func (p *Provider) FetchUser(session goth.Session) (goth.User, error) {
	msSession := session.(*Session)
	user := goth.User{
		AccessToken: msSession.AccessToken,
		Provider:    p.Name(),
		ExpiresAt:   msSession.ExpiresAt,
	}

	if user.AccessToken == "" {
		return user, fmt.Errorf("%s cannot get user information without accessToken", p.providerName)
	}

	req, err := http.NewRequest("GET", graphAPIResource+"me", nil)
	if err != nil {
		return user, err
	}

	req.Header.Set(authorizationHeader(msSession))

	response, err := p.Client().Do(req)
	if err != nil {
		return user, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return user, fmt.Errorf("%s responded with a %d trying to fetch user information", p.providerName, response.StatusCode)
	}

	err = userFromReader(response.Body, &user)
	user.AccessToken = msSession.AccessToken
	user.RefreshToken = msSession.RefreshToken
	user.ExpiresAt = msSession.ExpiresAt
	return user, err
}

//RefreshTokenAvailable refresh token is provided by auth provider or not
func (p *Provider) RefreshTokenAvailable() bool {
	return true
}

//RefreshToken get new access token based on the refresh token
func (p *Provider) RefreshToken(refreshToken string) (*oauth2.Token, error) {
	token := &oauth2.Token{RefreshToken: refreshToken}
	ts := p.config.TokenSource(goth.ContextForClient(p.Client()), token)
	newToken, err := ts.Token()
	if err != nil {
		return nil, err
	}
	return newToken, err
}

func authorizationHeader(session *Session) (string, string) {
	return "Authorization", fmt.Sprintf("Bearer %s", session.AccessToken)
}

func userFromReader(r io.Reader, user *goth.User) error {
	u := struct {
		ID                string   `json:"id"`                // The unique identifier for the user.
		BusinessPhones    []string `json:"businessPhones"`    // The user's phone numbers.
		DisplayName       string   `json:"displayName"`       // The name displayed in the address book for the user.
		FirstName         string   `json:"givenName"`         // The first name of the user.
		JobTitle          string   `json:"jobTitle"`          // The user's job title.
		Email             string   `json:"mail"`              // The user's email address.
		MobilePhone       string   `json:"mobilePhone"`       // The user's cellphone number.
		OfficeLocation    string   `json:"officeLocation"`    // The user's physical office location.
		PreferredLanguage string   `json:"preferredLanguage"` // The user's language of preference.
		LastName          string   `json:"surname"`           // The last name of the user.
		UserPrincipalName string   `json:"userPrincipalName"` // The user's principal name.
	}{}

	userBytes, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(userBytes, &u); err != nil {
		return err
	}

	user.Email = u.Email
	user.Name = u.DisplayName
	user.FirstName = u.FirstName
	user.LastName = u.LastName
	user.NickName = u.DisplayName
	user.Location = u.OfficeLocation
	user.UserID = u.ID
	user.AvatarURL = graphAPIResource + fmt.Sprintf("users/%s/photo/$value", u.ID)
	// Make sure all of the information returned is available via RawData
	if err := json.Unmarshal(userBytes, &user.RawData); err != nil {
		return err
	}

	return nil
}

func scopesToStrings(scopes ...ScopeType) []string {
	strs := make([]string, len(scopes))
	for i := 0; i < len(scopes); i++ {
		strs[i] = string(scopes[i])
	}
	return strs
}
