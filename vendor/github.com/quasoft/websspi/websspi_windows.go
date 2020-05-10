package websspi

import (
	"context"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/quasoft/websspi/secctx"
)

// The Config object determines the behaviour of the Authenticator.
type Config struct {
	contextStore    secctx.Store
	authAPI         API
	KrbPrincipal    string // Name of Kerberos principle used by the service (optional).
	AuthUserKey     string // Key of header to fill with authenticated username, eg. "X-Authenticated-User" or "REMOTE_USER" (optional).
	EnumerateGroups bool   // If true, groups the user is a member of are enumerated and stored in request context (default false)
	ServerName      string // Specifies the DNS or NetBIOS name of the remote server which to query about user groups. Ignored if EnumerateGroups is false.
}

// NewConfig creates a configuration object with default values.
func NewConfig() *Config {
	return &Config{
		contextStore: secctx.NewCookieStore(),
		authAPI:      &Win32{},
	}
}

// Validate makes basic validation of configuration to make sure that important and required fields
// have been set with values in expected format.
func (c *Config) Validate() error {
	if c.contextStore == nil {
		return errors.New("Store for context handles not specified in Config")
	}
	if c.authAPI == nil {
		return errors.New("Authentication API not specified in Config")
	}
	return nil
}

// contextKey represents a custom key for values stored in context.Context
type contextKey string

func (c contextKey) String() string {
	return "websspi-key-" + string(c)
}

var (
	UserInfoKey = contextKey("UserInfo")
)

// The Authenticator type provides middleware methods for authentication of http requests.
// A single authenticator object can be shared by concurrent goroutines.
type Authenticator struct {
	Config     Config
	serverCred *CredHandle
	credExpiry *time.Time
	ctxList    []CtxtHandle
	ctxListMux *sync.Mutex
}

// New creates a new Authenticator object with the given configuration options.
func New(config *Config) (*Authenticator, error) {
	err := config.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid config: %v", err)
	}

	var auth = &Authenticator{
		Config:     *config,
		ctxListMux: &sync.Mutex{},
	}

	err = auth.PrepareCredentials(config.KrbPrincipal)
	if err != nil {
		return nil, fmt.Errorf("could not acquire credentials handle for the service: %v", err)
	}
	log.Printf("Credential handle expiry: %v\n", *auth.credExpiry)

	return auth, nil
}

// PrepareCredentials method acquires a credentials handle for the specified principal
// for use during the live of the application.
// On success stores the handle in the serverCred field and its expiry time in the
// credExpiry field.
// This method must be called once - when the application is starting or when the first
// request from a client is received.
func (a *Authenticator) PrepareCredentials(principal string) error {
	var principalPtr *uint16
	if principal != "" {
		var err error
		principalPtr, err = syscall.UTF16PtrFromString(principal)
		if err != nil {
			return err
		}
	}
	credentialUsePtr, err := syscall.UTF16PtrFromString(NEGOSSP_NAME)
	if err != nil {
		return err
	}
	var handle CredHandle
	var expiry syscall.Filetime
	status := a.Config.authAPI.AcquireCredentialsHandle(
		principalPtr,
		credentialUsePtr,
		SECPKG_CRED_INBOUND,
		nil, // logonId
		nil, // authData
		0,   // getKeyFn
		0,   // getKeyArgument
		&handle,
		&expiry,
	)
	if status != SEC_E_OK {
		return fmt.Errorf("call to AcquireCredentialsHandle failed with code 0x%x", status)
	}
	expiryTime := time.Unix(0, expiry.Nanoseconds())
	a.credExpiry = &expiryTime
	a.serverCred = &handle
	return nil
}

// Free method should be called before shutting down the server to let
// it release allocated Win32 resources
func (a *Authenticator) Free() error {
	var status SECURITY_STATUS
	a.ctxListMux.Lock()
	for _, ctx := range a.ctxList {
		// TODO: Also check for stale security contexts and delete them periodically
		status = a.Config.authAPI.DeleteSecurityContext(&ctx)
		if status != SEC_E_OK {
			return fmt.Errorf("call to DeleteSecurityContext failed with code 0x%x", status)
		}
	}
	a.ctxList = nil
	a.ctxListMux.Unlock()
	if a.serverCred != nil {
		status = a.Config.authAPI.FreeCredentialsHandle(a.serverCred)
		if status != SEC_E_OK {
			return fmt.Errorf("call to FreeCredentialsHandle failed with code 0x%x", status)
		}
		a.serverCred = nil
	}
	return nil
}

// StoreCtxHandle stores the specified context to the internal list (ctxList)
func (a *Authenticator) StoreCtxHandle(handle *CtxtHandle) {
	if handle == nil || *handle == (CtxtHandle{}) {
		// Should not add nil or empty handle
		return
	}
	a.ctxListMux.Lock()
	defer a.ctxListMux.Unlock()
	a.ctxList = append(a.ctxList, *handle)
}

// ReleaseCtxHandle deletes a context handle and removes it from the internal list (ctxList)
func (a *Authenticator) ReleaseCtxHandle(handle *CtxtHandle) error {
	if handle == nil || *handle == (CtxtHandle{}) {
		// Removing a nil or empty handle is not an error condition
		return nil
	}
	a.ctxListMux.Lock()
	defer a.ctxListMux.Unlock()

	// First, try to delete the handle
	status := a.Config.authAPI.DeleteSecurityContext(handle)
	if status != SEC_E_OK {
		return fmt.Errorf("call to DeleteSecurityContext failed with code 0x%x", status)
	}

	// Then remove it from the internal list
	foundAt := -1
	for i, ctx := range a.ctxList {
		if ctx == *handle {
			foundAt = i
			break
		}
	}
	if foundAt > -1 {
		a.ctxList[foundAt] = a.ctxList[len(a.ctxList)-1]
		a.ctxList = a.ctxList[:len(a.ctxList)-1]
	}
	return nil
}

// AcceptOrContinue tries to validate the auth-data token by calling the AcceptSecurityContext
// function and returns and error if validation failed or continuation of the negotiation is needed.
// No error is returned if the token was validated (user was authenticated).
func (a *Authenticator) AcceptOrContinue(context *CtxtHandle, authData []byte) (newCtx *CtxtHandle, out []byte, exp *time.Time, err error) {
	if authData == nil {
		err = errors.New("input token cannot be nil")
		return
	}

	var inputDesc SecBufferDesc
	var inputBuf SecBuffer
	inputDesc.BuffersCount = 1
	inputDesc.Version = SECBUFFER_VERSION
	inputDesc.Buffers = &inputBuf
	inputBuf.BufferSize = uint32(len(authData))
	inputBuf.BufferType = SECBUFFER_TOKEN
	inputBuf.Buffer = &authData[0]

	var outputDesc SecBufferDesc
	var outputBuf SecBuffer
	outputDesc.BuffersCount = 1
	outputDesc.Version = SECBUFFER_VERSION
	outputDesc.Buffers = &outputBuf
	outputBuf.BufferSize = 0
	outputBuf.BufferType = SECBUFFER_TOKEN
	outputBuf.Buffer = nil

	var expiry syscall.Filetime
	var contextAttr uint32
	var newContextHandle CtxtHandle

	var status = a.Config.authAPI.AcceptSecurityContext(
		a.serverCred,
		context,
		&inputDesc,
		ASC_REQ_ALLOCATE_MEMORY|ASC_REQ_MUTUAL_AUTH|ASC_REQ_CONFIDENTIALITY|
			ASC_REQ_INTEGRITY|ASC_REQ_REPLAY_DETECT|ASC_REQ_SEQUENCE_DETECT, // contextReq uint32,
		SECURITY_NATIVE_DREP, // targDataRep uint32,
		&newContextHandle,
		&outputDesc,  // *SecBufferDesc
		&contextAttr, // contextAttr *uint32,
		&expiry,      // *syscall.Filetime
	)
	if newContextHandle.Lower != 0 || newContextHandle.Upper != 0 {
		newCtx = &newContextHandle
	}
	tm := time.Unix(0, expiry.Nanoseconds())
	exp = &tm
	if status == SEC_E_OK || status == SEC_I_CONTINUE_NEEDED {
		// Copy outputBuf.Buffer to out and free the outputBuf.Buffer
		out = make([]byte, outputBuf.BufferSize)
		var bufPtr = uintptr(unsafe.Pointer(outputBuf.Buffer))
		for i := 0; i < len(out); i++ {
			out[i] = *(*byte)(unsafe.Pointer(bufPtr))
			bufPtr++
		}
	}
	if outputBuf.Buffer != nil {
		freeStatus := a.Config.authAPI.FreeContextBuffer(outputBuf.Buffer)
		if freeStatus != SEC_E_OK {
			status = freeStatus
			err = fmt.Errorf("could not free output buffer; FreeContextBuffer() failed with code: 0x%x", freeStatus)
			return
		}
	}
	if status == SEC_I_CONTINUE_NEEDED {
		err = errors.New("Negotiation should continue")
		return
	} else if status != SEC_E_OK {
		err = fmt.Errorf("call to AcceptSecurityContext failed with code 0x%x", status)
		return
	}
	// TODO: Check contextAttr?
	return
}

// GetCtxHandle retrieves the context handle for this client from request's cookies
func (a *Authenticator) GetCtxHandle(r *http.Request) (*CtxtHandle, error) {
	sessionHandle, err := a.Config.contextStore.GetHandle(r)
	if err != nil {
		return nil, fmt.Errorf("could not get context handle from session: %s", err)
	}
	if contextHandle, ok := sessionHandle.(*CtxtHandle); ok {
		log.Printf("CtxHandle: 0x%x\n", *contextHandle)
		if contextHandle.Lower == 0 && contextHandle.Upper == 0 {
			return nil, nil
		}
		return contextHandle, nil
	}
	log.Printf("CtxHandle: nil\n")
	return nil, nil
}

// SetCtxHandle stores the context handle for this client to cookie of response
func (a *Authenticator) SetCtxHandle(r *http.Request, w http.ResponseWriter, newContext *CtxtHandle) error {
	// Store can't store nil value, so if newContext is nil, store an empty CtxHandle
	ctx := &CtxtHandle{}
	if newContext != nil {
		ctx = newContext
	}
	err := a.Config.contextStore.SetHandle(r, w, ctx)
	if err != nil {
		return fmt.Errorf("could not save context to cookie: %s", err)
	}
	log.Printf("New context: 0x%x\n", *ctx)
	return nil
}

// GetFlags returns the negotiated context flags
func (a *Authenticator) GetFlags(context *CtxtHandle) (uint32, error) {
	var flags SecPkgContext_Flags
	status := a.Config.authAPI.QueryContextAttributes(context, SECPKG_ATTR_FLAGS, (*byte)(unsafe.Pointer(&flags)))
	if status != SEC_E_OK {
		return 0, fmt.Errorf("QueryContextAttributes failed with status 0x%x", status)
	}
	return flags.Flags, nil
}

// GetUsername returns the name of the user associated with the specified security context
func (a *Authenticator) GetUsername(context *CtxtHandle) (username string, err error) {
	var names SecPkgContext_Names
	status := a.Config.authAPI.QueryContextAttributes(context, SECPKG_ATTR_NAMES, (*byte)(unsafe.Pointer(&names)))
	if status != SEC_E_OK {
		err = fmt.Errorf("QueryContextAttributes failed with status 0x%x", status)
		return
	}
	if names.UserName != nil {
		username = UTF16PtrToString(names.UserName, 2048)
		status = a.Config.authAPI.FreeContextBuffer((*byte)(unsafe.Pointer(names.UserName)))
		if status != SEC_E_OK {
			err = fmt.Errorf("FreeContextBuffer failed with status 0x%x", status)
		}
		return
	}
	err = errors.New("QueryContextAttributes returned empty name")
	return
}

// GetUserGroups returns the groups the user is a member of
func (a *Authenticator) GetUserGroups(userName string) (groups []string, err error) {
	var serverNamePtr *uint16
	if a.Config.ServerName != "" {
		serverNamePtr, err = syscall.UTF16PtrFromString(a.Config.ServerName)
		if err != nil {
			return
		}
	}

	userNamePtr, err := syscall.UTF16PtrFromString(userName)
	if err != nil {
		return
	}
	var buf *byte
	var entriesRead uint32
	var totalEntries uint32
	err = a.Config.authAPI.NetUserGetGroups(
		serverNamePtr,
		userNamePtr,
		0,
		&buf,
		MAX_PREFERRED_LENGTH,
		&entriesRead,
		&totalEntries,
	)
	if buf == nil {
		err = fmt.Errorf("NetUserGetGroups(): returned nil buffer, error: %s", err)
		return
	}
	defer func() {
		freeErr := a.Config.authAPI.NetApiBufferFree(buf)
		if freeErr != nil {
			err = freeErr
		}
	}()
	if err != nil {
		return
	}
	if entriesRead < totalEntries {
		err = fmt.Errorf("NetUserGetGroups(): could not read all entries, read only %d entries of %d", entriesRead, totalEntries)
		return
	}

	ptr := uintptr(unsafe.Pointer(buf))
	for i := uint32(0); i < entriesRead; i++ {
		groupInfo := (*GroupUsersInfo0)(unsafe.Pointer(ptr))
		groupName := UTF16PtrToString(groupInfo.Grui0_name, MAX_GROUP_NAME_LENGTH)
		if groupName != "" {
			groups = append(groups, groupName)
		}
		ptr += unsafe.Sizeof(GroupUsersInfo0{})
	}
	return
}

// GetUserInfo returns a structure containing the name of the user associated with the
// specified security context and the groups to which they are a member of (if Config.EnumerateGroups)
// is enabled
func (a *Authenticator) GetUserInfo(context *CtxtHandle) (*UserInfo, error) {
	// Get username
	username, err := a.GetUsername(context)
	if err != nil {
		return nil, err
	}
	info := UserInfo{
		Username: username,
	}

	// Get groups
	if a.Config.EnumerateGroups {
		info.Groups, err = a.GetUserGroups(username)
		if err != nil {
			return nil, err
		}
	}

	return &info, nil
}

// GetAuthData parses the "Authorization" header received from the client,
// extracts the auth-data token (input token) and decodes it to []byte
func (a *Authenticator) GetAuthData(r *http.Request, w http.ResponseWriter) (authData []byte, err error) {
	// 1. Check if Authorization header is present
	headers := r.Header["Authorization"]
	if len(headers) == 0 {
		err = errors.New("the Authorization header is not provided")
		return
	}
	if len(headers) > 1 {
		err = errors.New("received multiple Authorization headers, but expected only one")
		return
	}

	authzHeader := strings.TrimSpace(headers[0])
	if authzHeader == "" {
		err = errors.New("the Authorization header is empty")
		return
	}
	// 1.1. Make sure header starts with "Negotiate"
	if !strings.HasPrefix(strings.ToLower(authzHeader), "negotiate") {
		err = errors.New("the Authorization header does not start with 'Negotiate'")
		return
	}

	// 2. Extract token from Authorization header
	authzParts := strings.Split(authzHeader, " ")
	if len(authzParts) < 2 {
		err = errors.New("the Authorization header does not contain token (gssapi-data)")
		return
	}
	token := authzParts[len(authzParts)-1]
	if token == "" {
		err = errors.New("the token (gssapi-data) in the Authorization header is empty")
		return
	}

	// 3. Decode token
	authData, err = base64.StdEncoding.DecodeString(token)
	if err != nil {
		err = errors.New("could not decode token as base64 string")
		return
	}

	return
}

// Authenticate tries to authenticate the HTTP request and returns nil
// if authentication was successful.
// Returns error and data for continuation if authentication was not successful.
func (a *Authenticator) Authenticate(r *http.Request, w http.ResponseWriter) (userInfo *UserInfo, outToken string, err error) {
	// 1. Extract auth-data from Authorization header
	authData, err := a.GetAuthData(r, w)
	if err != nil {
		err = fmt.Errorf("could not get auth data: %s", err)
		return
	}

	// 2. Authenticate user with provided token
	contextHandle, err := a.GetCtxHandle(r)
	if err != nil {
		return
	}
	newCtx, output, _, err := a.AcceptOrContinue(contextHandle, authData)

	// If a new context was created, make sure to delete it or store it
	// both in internal list and response Cookie
	defer func() {
		// Negotiation is ending if we don't expect further responses from the client
		// (authentication was successful or no output token is going to be sent back),
		// clear client cookie
		endOfNegotiation := err == nil || len(output) == 0

		// Current context (contextHandle) is not needed anymore and should be deleted if:
		// - we don't expect further responses from the client
		// - a new context has been returned by AcceptSecurityContext
		currCtxNotNeeded := endOfNegotiation || newCtx != nil
		if !currCtxNotNeeded {
			// Release current context only if its different than the new context
			if contextHandle != nil && *contextHandle != *newCtx {
				remErr := a.ReleaseCtxHandle(contextHandle)
				if remErr != nil {
					err = remErr
					return
				}
			}
		}

		if endOfNegotiation {
			// Clear client cookie
			setErr := a.SetCtxHandle(r, w, nil)
			if setErr != nil {
				err = fmt.Errorf("could not clear context, error: %s", setErr)
				return
			}

			// Delete any new context handle
			remErr := a.ReleaseCtxHandle(newCtx)
			if remErr != nil {
				err = remErr
				return
			}

			// Exit defer func
			return
		}

		if newCtx != nil {
			// Store new context handle to internal list and response Cookie
			a.StoreCtxHandle(newCtx)
			setErr := a.SetCtxHandle(r, w, newCtx)
			if setErr != nil {
				err = setErr
				return
			}
		}
	}()

	outToken = base64.StdEncoding.EncodeToString(output)
	if err != nil {
		err = fmt.Errorf("AcceptOrContinue failed: %s", err)
		return
	}

	// 3. Get username and user groups
	currentCtx := newCtx
	if currentCtx == nil {
		currentCtx = contextHandle
	}
	userInfo, err = a.GetUserInfo(currentCtx)
	if err != nil {
		err = fmt.Errorf("could not get username, error: %s", err)
		return
	}

	return
}

// AppendAuthenticateHeader populates WWW-Authenticate header,
// indicating to client that authentication is required and returns a 401 (Unauthorized)
// response code.
// The data parameter can be empty for the first 401 response from the server.
// For subsequent 401 responses the data parameter should contain the gssapi-data,
// which is required for continuation of the negotiation.
func (a *Authenticator) AppendAuthenticateHeader(w http.ResponseWriter, data string) {
	value := "Negotiate"
	if data != "" {
		value += " " + data
	}
	w.Header().Set("WWW-Authenticate", value)
}

// Return401 populates WWW-Authenticate header, indicating to client that authentication
// is required and returns a 401 (Unauthorized) response code.
// The data parameter can be empty for the first 401 response from the server.
// For subsequent 401 responses the data parameter should contain the gssapi-data,
// which is required for continuation of the negotiation.
func (a *Authenticator) Return401(w http.ResponseWriter, data string) {
	a.AppendAuthenticateHeader(w, data)
	http.Error(w, "Error!", http.StatusUnauthorized)
}

// WithAuth authenticates the request. On successful authentication the request
// is passed down to the next http handler. The next handler can access information
// about the authenticated user via the GetUserName method.
// If authentication was not successful, the server returns 401 response code with
// a WWW-Authenticate, indicating that authentication is required.
func (a *Authenticator) WithAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Authenticating request to %s\n", r.RequestURI)

		user, data, err := a.Authenticate(r, w)
		if err != nil {
			log.Printf("Authentication failed with error: %v\n", err)
			a.Return401(w, data)
			return
		}

		log.Print("Authenticated\n")
		// Add the UserInfo value to the reqest's context
		r = r.WithContext(context.WithValue(r.Context(), UserInfoKey, user))
		// and to the request header with key Config.AuthUserKey
		if a.Config.AuthUserKey != "" {
			r.Header.Set(a.Config.AuthUserKey, user.Username)
		}

		// The WWW-Authenticate header might need to be sent back even
		// on successful authentication (eg. in order to let the client complete
		// mutual authentication).
		if data != "" {
			a.AppendAuthenticateHeader(w, data)
		}
		next.ServeHTTP(w, r)
	})
}

func init() {
	gob.Register(&CtxtHandle{})
	gob.Register(&UserInfo{})
}
