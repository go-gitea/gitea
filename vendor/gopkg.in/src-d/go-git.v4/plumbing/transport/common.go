// Package transport includes the implementation for different transport
// protocols.
//
// `Client` can be used to fetch and send packfiles to a git server.
// The `client` package provides higher level functions to instantiate the
// appropriate `Client` based on the repository URL.
//
// go-git supports HTTP and SSH (see `Protocols`), but you can also install
// your own protocols (see the `client` package).
//
// Each protocol has its own implementation of `Client`, but you should
// generally not use them directly, use `client.NewClient` instead.
package transport

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/capability"
)

var (
	ErrRepositoryNotFound     = errors.New("repository not found")
	ErrEmptyRemoteRepository  = errors.New("remote repository is empty")
	ErrAuthorizationRequired  = errors.New("authorization required")
	ErrEmptyUploadPackRequest = errors.New("empty git-upload-pack given")
	ErrInvalidAuthMethod      = errors.New("invalid auth method")
)

const (
	UploadPackServiceName  = "git-upload-pack"
	ReceivePackServiceName = "git-receive-pack"
)

// Client can initiate git-fetch-pack and git-send-pack processes.
type Client interface {
	// NewFetchPackSession starts a git-fetch-pack session for an endpoint.
	NewFetchPackSession(Endpoint) (FetchPackSession, error)
	// NewSendPackSession starts a git-send-pack session for an endpoint.
	NewSendPackSession(Endpoint) (SendPackSession, error)
}

type Session interface {
	SetAuth(auth AuthMethod) error
	// AdvertisedReferences retrieves the advertised references for a
	// repository.
	// If the repository does not exist, returns ErrRepositoryNotFound.
	// If the repository exists, but is empty, returns ErrEmptyRemoteRepository.
	AdvertisedReferences() (*packp.AdvRefs, error)
	io.Closer
}

type AuthMethod interface {
	fmt.Stringer
	Name() string
}

// FetchPackSession represents a git-fetch-pack session.
// A git-fetch-pack session has two steps: reference discovery
// (`AdvertisedReferences` function) and fetching pack (`FetchPack` function).
// In that order.
type FetchPackSession interface {
	Session
	// FetchPack takes a request and returns a reader for the packfile
	// received from the server.
	FetchPack(*packp.UploadPackRequest) (*packp.UploadPackResponse, error)
}

// SendPackSession represents a git-send-pack session.
// A git-send-pack session has two steps: reference discovery
// (`AdvertisedReferences` function) and sending pack (`SendPack` function).
// In that order.
type SendPackSession interface {
	Session
	// UpdateReferences sends an update references request and a packfile
	// reader and returns a ReportStatus and error.
	SendPack(*packp.ReferenceUpdateRequest) (*packp.ReportStatus, error)
}

type Endpoint url.URL

var (
	isSchemeRegExp   = regexp.MustCompile("^[^:]+://")
	scpLikeUrlRegExp = regexp.MustCompile("^(?P<user>[^@]+@)?(?P<host>[^:]+):/?(?P<path>.+)$")
)

func NewEndpoint(endpoint string) (Endpoint, error) {
	endpoint = transformSCPLikeIfNeeded(endpoint)

	u, err := url.Parse(endpoint)
	if err != nil {
		return Endpoint{}, plumbing.NewPermanentError(err)
	}

	if !u.IsAbs() {
		return Endpoint{}, plumbing.NewPermanentError(fmt.Errorf(
			"invalid endpoint: %s", endpoint,
		))
	}

	return Endpoint(*u), nil
}

func (e *Endpoint) String() string {
	u := url.URL(*e)
	return u.String()
}

func transformSCPLikeIfNeeded(endpoint string) string {
	if !isSchemeRegExp.MatchString(endpoint) && scpLikeUrlRegExp.MatchString(endpoint) {
		m := scpLikeUrlRegExp.FindStringSubmatch(endpoint)
		return fmt.Sprintf("ssh://%s%s/%s", m[1], m[2], m[3])
	}

	return endpoint
}

// UnsupportedCapabilities are the capabilities not supported by any client
// implementation
var UnsupportedCapabilities = []capability.Capability{
	capability.MultiACK,
	capability.MultiACKDetailed,
	capability.ThinPack,
}

// FilterUnsupportedCapabilities it filter out all the UnsupportedCapabilities
// from a capability.List, the intended usage is on the client implementation
// to filter the capabilities from an AdvRefs message.
func FilterUnsupportedCapabilities(list *capability.List) {
	for _, c := range UnsupportedCapabilities {
		list.Delete(c)
	}
}
