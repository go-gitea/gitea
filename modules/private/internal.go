// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func newRequest(url, method string) *httplib.Request {
	return httplib.NewRequest(url, method).Header("Authorization",
		fmt.Sprintf("Bearer %s", setting.InternalToken))
}

// Response internal request response
type Response struct {
	Err string `json:"err"`
}

func decodeJSONError(resp *http.Response) *Response {
	var res Response
	err := json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		res.Err = err.Error()
	}
	return &res
}

func newInternalRequest(url, method string) *httplib.Request {
	req := newRequest(url, method).SetTLSClientConfig(&tls.Config{
		InsecureSkipVerify: true,
	})
	if setting.Protocol == setting.UnixSocket {
		req.SetTransport(&http.Transport{
			Dial: func(_, _ string) (net.Conn, error) {
				return net.Dial("unix", setting.HTTPAddr)
			},
		})
	}
	return req
}

// UpdatePublicKeyUpdated update publick key updates
func UpdatePublicKeyUpdated(keyID int64) error {
	// Ask for running deliver hook and test pull request tasks.
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/ssh/%d/update", keyID)
	log.GitLogger.Trace("UpdatePublicKeyUpdated: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "POST").Response()
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Failed to update public key: %s", decodeJSONError(resp).Err)
	}
	return nil
}
