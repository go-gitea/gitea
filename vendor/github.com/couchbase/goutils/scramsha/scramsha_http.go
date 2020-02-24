// @author Couchbase <info@couchbase.com>
// @copyright 2018 Couchbase, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package scramsha provides implementation of client side SCRAM-SHA
// via Http according to https://tools.ietf.org/html/rfc7804
package scramsha

import (
	"encoding/base64"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

// consts used to parse scramsha response from target
const (
	WWWAuthenticate    = "WWW-Authenticate"
	AuthenticationInfo = "Authentication-Info"
	Authorization      = "Authorization"
	DataPrefix         = "data="
	SidPrefix          = "sid="
)

// Request provides implementation of http request that can be retried
type Request struct {
	body io.ReadSeeker

	// Embed an HTTP request directly. This makes a *Request act exactly
	// like an *http.Request so that all meta methods are supported.
	*http.Request
}

type lenReader interface {
	Len() int
}

// NewRequest creates http request that can be retried
func NewRequest(method, url string, body io.ReadSeeker) (*Request, error) {
	// Wrap the body in a noop ReadCloser if non-nil. This prevents the
	// reader from being closed by the HTTP client.
	var rcBody io.ReadCloser
	if body != nil {
		rcBody = ioutil.NopCloser(body)
	}

	// Make the request with the noop-closer for the body.
	httpReq, err := http.NewRequest(method, url, rcBody)
	if err != nil {
		return nil, err
	}

	// Check if we can set the Content-Length automatically.
	if lr, ok := body.(lenReader); ok {
		httpReq.ContentLength = int64(lr.Len())
	}

	return &Request{body, httpReq}, nil
}

func encode(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

func decode(str string) (string, error) {
	bytes, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return "", errors.Errorf("Cannot base64 decode %s",
			str)
	}
	return string(bytes), err
}

func trimPrefix(s, prefix string) (string, error) {
	l := len(s)
	trimmed := strings.TrimPrefix(s, prefix)
	if l == len(trimmed) {
		return trimmed, errors.Errorf("Prefix %s not found in %s",
			prefix, s)
	}
	return trimmed, nil
}

func drainBody(resp *http.Response) {
	defer resp.Body.Close()
	io.Copy(ioutil.Discard, resp.Body)
}

// DoScramSha performs SCRAM-SHA handshake via Http
func DoScramSha(req *Request,
	username string,
	password string,
	client *http.Client) (*http.Response, error) {

	method := "SCRAM-SHA-512"
	s, err := NewScramSha("SCRAM-SHA512")
	if err != nil {
		return nil, errors.Wrap(err,
			"Unable to initialize SCRAM-SHA handler")
	}

	message, err := s.GetStartRequest(username)
	if err != nil {
		return nil, err
	}

	encodedMessage := method + " " + DataPrefix + encode(message)

	req.Header.Set(Authorization, encodedMessage)

	res, err := client.Do(req.Request)
	if err != nil {
		return nil, errors.Wrap(err, "Problem sending SCRAM-SHA start"+
			"request")
	}

	if res.StatusCode != http.StatusUnauthorized {
		return res, nil
	}

	authHeader := res.Header.Get(WWWAuthenticate)
	if authHeader == "" {
		drainBody(res)
		return nil, errors.Errorf("Header %s is not populated in "+
			"SCRAM-SHA start response", WWWAuthenticate)
	}

	authHeader, err = trimPrefix(authHeader, method+" ")
	if err != nil {
		if strings.HasPrefix(authHeader, "Basic ") {
			// user not found
			return res, nil
		}
		drainBody(res)
		return nil, errors.Wrapf(err, "Error while parsing SCRAM-SHA "+
			"start response %s", authHeader)
	}

	drainBody(res)

	sid, response, err := parseSidAndData(authHeader)
	if err != nil {
		return nil, errors.Wrapf(err, "Error while parsing SCRAM-SHA "+
			"start response %s", authHeader)
	}

	err = s.HandleStartResponse(response)
	if err != nil {
		return nil, errors.Wrapf(err, "Error parsing SCRAM-SHA start "+
			"response %s", response)
	}

	message = s.GetFinalRequest(password)
	encodedMessage = method + " " + SidPrefix + sid + "," + DataPrefix +
		encode(message)

	req.Header.Set(Authorization, encodedMessage)

	// rewind request body so it can be resent again
	if req.body != nil {
		if _, err = req.body.Seek(0, 0); err != nil {
			return nil, errors.Errorf("Failed to seek body: %v",
				err)
		}
	}

	res, err = client.Do(req.Request)
	if err != nil {
		return nil, errors.Wrap(err, "Problem sending SCRAM-SHA final"+
			"request")
	}

	if res.StatusCode == http.StatusUnauthorized {
		// TODO retrieve and return error
		return res, nil
	}

	if res.StatusCode >= http.StatusInternalServerError {
		// in this case we cannot expect server to set headers properly
		return res, nil
	}

	authHeader = res.Header.Get(AuthenticationInfo)
	if authHeader == "" {
		drainBody(res)
		return nil, errors.Errorf("Header %s is not populated in "+
			"SCRAM-SHA final response", AuthenticationInfo)
	}

	finalSid, response, err := parseSidAndData(authHeader)
	if err != nil {
		drainBody(res)
		return nil, errors.Wrapf(err, "Error while parsing SCRAM-SHA "+
			"final response %s", authHeader)
	}

	if finalSid != sid {
		drainBody(res)
		return nil, errors.Errorf("Sid %s returned by server "+
			"doesn't match the original sid %s", finalSid, sid)
	}

	err = s.HandleFinalResponse(response)
	if err != nil {
		drainBody(res)
		return nil, errors.Wrapf(err,
			"Error handling SCRAM-SHA final server response %s",
			response)
	}
	return res, nil
}

func parseSidAndData(authHeader string) (string, string, error) {
	sidIndex := strings.Index(authHeader, SidPrefix)
	if sidIndex < 0 {
		return "", "", errors.Errorf("Cannot find %s in %s",
			SidPrefix, authHeader)
	}

	sidEndIndex := strings.Index(authHeader, ",")
	if sidEndIndex < 0 {
		return "", "", errors.Errorf("Cannot find ',' in %s",
			authHeader)
	}

	sid := authHeader[sidIndex+len(SidPrefix) : sidEndIndex]

	dataIndex := strings.Index(authHeader, DataPrefix)
	if dataIndex < 0 {
		return "", "", errors.Errorf("Cannot find %s in %s",
			DataPrefix, authHeader)
	}

	data, err := decode(authHeader[dataIndex+len(DataPrefix):])
	if err != nil {
		return "", "", err
	}
	return sid, data, nil
}
