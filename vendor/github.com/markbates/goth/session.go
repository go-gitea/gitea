package goth

// Params is used to pass data to sessions for authorization. An existing
// implementation, and the one most likely to be used, is `url.Values`.
type Params interface {
	Get(string) string
}

// Session needs to be implemented as part of the provider package.
// It will be marshaled and persisted between requests to "tie"
// the start and the end of the authorization process with a
// 3rd party provider.
type Session interface {
	// GetAuthURL returns the URL for the authentication end-point for the provider.
	GetAuthURL() (string, error)
	// Marshal generates a string representation of the Session for storing between requests.
	Marshal() string
	// Authorize should validate the data from the provider and return an access token
	// that can be stored for later access to the provider.
	Authorize(Provider, Params) (string, error)
}
