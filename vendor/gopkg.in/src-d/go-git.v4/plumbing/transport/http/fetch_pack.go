package http

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/pktline"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/internal/common"
	"gopkg.in/src-d/go-git.v4/utils/ioutil"
)

type fetchPackSession struct {
	*session
}

func newFetchPackSession(c *http.Client,
	ep transport.Endpoint) transport.FetchPackSession {

	return &fetchPackSession{
		session: &session{
			auth:     basicAuthFromEndpoint(ep),
			client:   c,
			endpoint: ep,
		},
	}
}

func (s *fetchPackSession) AdvertisedReferences() (*packp.AdvRefs, error) {
	if s.advRefs != nil {
		return s.advRefs, nil
	}

	url := fmt.Sprintf(
		"%s/info/refs?service=%s",
		s.endpoint.String(), transport.UploadPackServiceName,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	s.applyAuthToRequest(req)
	s.applyHeadersToRequest(req, nil)
	res, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode == http.StatusUnauthorized {
		return nil, transport.ErrAuthorizationRequired
	}

	ar := packp.NewAdvRefs()
	if err := ar.Decode(res.Body); err != nil {
		if err == packp.ErrEmptyAdvRefs {
			err = transport.ErrEmptyRemoteRepository
		}

		return nil, err
	}

	transport.FilterUnsupportedCapabilities(ar.Capabilities)
	s.advRefs = ar
	return ar, nil
}

func (s *fetchPackSession) FetchPack(req *packp.UploadPackRequest) (*packp.UploadPackResponse, error) {
	if req.IsEmpty() {
		return nil, transport.ErrEmptyUploadPackRequest
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf(
		"%s/%s",
		s.endpoint.String(), transport.UploadPackServiceName,
	)

	content, err := uploadPackRequestToReader(req)
	if err != nil {
		return nil, err
	}

	res, err := s.doRequest(http.MethodPost, url, content)
	if err != nil {
		return nil, err
	}

	r, err := ioutil.NonEmptyReader(res.Body)
	if err != nil {
		if err == ioutil.ErrEmptyReader || err == io.ErrUnexpectedEOF {
			return nil, transport.ErrEmptyUploadPackRequest
		}

		return nil, err
	}

	rc := ioutil.NewReadCloser(r, res.Body)
	return common.DecodeUploadPackResponse(rc, req)
}

// Close does nothing.
func (s *fetchPackSession) Close() error {
	return nil
}

func (s *fetchPackSession) doRequest(method, url string, content *bytes.Buffer) (*http.Response, error) {
	var body io.Reader
	if content != nil {
		body = content
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, plumbing.NewPermanentError(err)
	}

	s.applyHeadersToRequest(req, content)
	s.applyAuthToRequest(req)

	res, err := s.client.Do(req)
	if err != nil {
		return nil, plumbing.NewUnexpectedError(err)
	}

	if err := NewErr(res); err != nil {
		_ = res.Body.Close()
		return nil, err
	}

	return res, nil
}

// it requires a bytes.Buffer, because we need to know the length
func (s *fetchPackSession) applyHeadersToRequest(req *http.Request, content *bytes.Buffer) {
	req.Header.Add("User-Agent", "git/1.0")
	req.Header.Add("Host", s.endpoint.Host)

	if content == nil {
		req.Header.Add("Accept", "*/*")
		return
	}

	req.Header.Add("Accept", "application/x-git-upload-pack-result")
	req.Header.Add("Content-Type", "application/x-git-upload-pack-request")
	req.Header.Add("Content-Length", strconv.Itoa(content.Len()))
}

func uploadPackRequestToReader(req *packp.UploadPackRequest) (*bytes.Buffer, error) {
	buf := bytes.NewBuffer(nil)
	e := pktline.NewEncoder(buf)

	if err := req.UploadRequest.Encode(buf); err != nil {
		return nil, fmt.Errorf("sending upload-req message: %s", err)
	}

	if err := req.UploadHaves.Encode(buf); err != nil {
		return nil, fmt.Errorf("sending haves message: %s", err)
	}

	_ = e.EncodeString("done\n")
	return buf, nil
}
