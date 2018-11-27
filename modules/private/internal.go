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

	"code.gitea.io/gitea/models"
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

// CheckUnitUser check whether user could visit the unit of this repository
func CheckUnitUser(userID, repoID int64, isAdmin bool, unitType models.UnitType) (bool, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/repositories/%d/user/%d/checkunituser?isAdmin=%t&unitType=%d", repoID, userID, isAdmin, unitType)
	log.GitLogger.Trace("AccessLevel: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return true, nil
	}
	return false, nil
}

// AccessLevel returns the Access a user has to a repository. Will return NoneAccess if the
// user does not have access.
func AccessLevel(userID, repoID int64) (*models.AccessMode, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/repositories/%d/user/%d/accesslevel", repoID, userID)
	log.GitLogger.Trace("AccessLevel: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to get user access level: %s", decodeJSONError(resp).Err)
	}

	var a models.AccessMode
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		return nil, err
	}

	return &a, nil
}

// GetRepositoryByOwnerAndName returns the repository by given ownername and reponame.
func GetRepositoryByOwnerAndName(ownerName, repoName string) (*models.Repository, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/repo/%s/%s", ownerName, repoName)
	log.GitLogger.Trace("GetRepositoryByOwnerAndName: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to get repository: %s", decodeJSONError(resp).Err)
	}

	var repo models.Repository
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return nil, err
	}

	return &repo, nil
}
