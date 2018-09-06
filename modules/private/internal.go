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

//TODO move on specific file

// UpdatePublicKeyUpdated update publick key updates
func UpdateDeployKeyUpdated(keyID int64, repoID int64) error {
	// Ask for running deliver hook and test pull request tasks.
	reqURL := setting.LocalURL + fmt.Sprintf("/repositories/%d/keys/%d/update", keyID, repoID)
	log.GitLogger.Trace("UpdateDeployKeyUpdated: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "POST").Response()
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Failed to update deploy key: %s", decodeJSONError(resp).Err)
	}
	return nil
}

/*
func GetDeployKeyByRepo(keyID, repoID int64) (*models.DeployKey, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/repositories/%d/keys/%d", repoID, keyID)
	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to get repository: %s", decodeJSONError(resp).Err)
	}

	var key models.DeployKey
	if err := json.NewDecoder(resp.Body).Decode(&key); err != nil {
		return nil, err
	}
	return &key, nil
}
*/
func HasDeployKey(keyID, repoID int64) (bool, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/repositories/%d/has-keys/%d", repoID, keyID)
	log.GitLogger.Trace("HasDeployKey: %s", reqURL)

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

func GetPublicKeyByID(keyID int64) (*models.PublicKey, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/ssh/%d", keyID)
	log.GitLogger.Trace("GetPublicKeyByID: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to get repository: %s", decodeJSONError(resp).Err)
	}

	var pKey models.PublicKey
	if err := json.NewDecoder(resp.Body).Decode(&pKey); err != nil {
		return nil, err
	}
	return &pKey, nil
}

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

func GetUserByKeyID(keyID int64) (*models.User, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/ssh/%d/user", keyID)
	log.GitLogger.Trace("GetUserByKeyID: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to get user: %s", decodeJSONError(resp).Err)
	}

	var user models.User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

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
