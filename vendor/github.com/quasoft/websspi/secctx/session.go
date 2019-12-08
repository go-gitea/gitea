package secctx

import (
	"net/http"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

// CookieStore can store and retrieve SSPI context handles to/from an encrypted Cookie.
type CookieStore struct {
	store *sessions.CookieStore
}

// NewCookieStore creates a new CookieStore for storing and retrieving of SSPI context handles
// to/from encrypted Cookies
func NewCookieStore() *CookieStore {
	s := &CookieStore{}
	s.store = sessions.NewCookieStore([]byte(securecookie.GenerateRandomKey(32)))
	return s
}

// GetHandle retrieves a *websspi.CtxtHandle value from the store
func (s *CookieStore) GetHandle(r *http.Request) (interface{}, error) {
	session, _ := s.store.Get(r, "websspi")
	contextHandle := session.Values["contextHandle"]
	return contextHandle, nil
}

// SetHandle saves a *websspi.CtxtHandle value to the store
func (s *CookieStore) SetHandle(r *http.Request, w http.ResponseWriter, contextHandle interface{}) error {
	session, _ := s.store.Get(r, "websspi")
	session.Values["contextHandle"] = contextHandle
	err := session.Save(r, w)
	return err
}
