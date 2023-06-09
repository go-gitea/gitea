package saml

import (
	"fmt"
	"net/http"
)

// Callout redirects request/response pair to authenticate against the provider
func (source *Source) Callout(request *http.Request, response http.ResponseWriter) error {
	samlRWMutex.RLock()
	defer samlRWMutex.RUnlock()
	if _, ok := providers[source.authSource.Name]; !ok {
		return fmt.Errorf("no provider for this saml")
	}

	authURL, err := providers[source.authSource.Name].samlSP.BuildAuthURL("")
	if err == nil {
		http.Redirect(response, request, authURL, http.StatusTemporaryRedirect)
	}
	return err
}

// Callback handles SAML callback, resolve to a goth user and send back to original url
// this will trigger a new authentication request, but because we save it in the session we can use that
func (source *Source) Callback(request *http.Request, response http.ResponseWriter) (error, error) {
	samlRWMutex.RLock()
	defer samlRWMutex.RUnlock()

	// TODO: complete
	return nil, nil
}
