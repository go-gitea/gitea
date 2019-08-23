// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"code.gitea.io/gitea/modules/setting"
)

// Git environment variables
const (
	GitAlternativeObjectDirectories = "GIT_ALTERNATE_OBJECT_DIRECTORIES"
	GitObjectDirectory              = "GIT_OBJECT_DIRECTORY"
	GitQuarantinePath               = "GIT_QUARANTINE_PATH"
)

// HookOptions represents the options for the Hook calls
type HookOptions struct {
	OldCommitID                     string
	NewCommitID                     string
	RefFullName                     string
	UserID                          int64
	UserName                        string
	GitObjectDirectory              string
	GitAlternativeObjectDirectories string
	GitQuarantinePath               string
	ProtectedBranchID               int64
}

// HookPreReceive check whether the provided commits are allowed
func HookPreReceive(ownerName, repoName string, opts HookOptions) (int, string) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/hook/pre-receive/%s/%s?old=%s&new=%s&ref=%s&userID=%d&gitObjectDirectory=%s&gitAlternativeObjectDirectories=%s&gitQuarantinePath=%s&prID=%d",
		url.PathEscape(ownerName),
		url.PathEscape(repoName),
		url.QueryEscape(opts.OldCommitID),
		url.QueryEscape(opts.NewCommitID),
		url.QueryEscape(opts.RefFullName),
		opts.UserID,
		url.QueryEscape(opts.GitObjectDirectory),
		url.QueryEscape(opts.GitAlternativeObjectDirectories),
		url.QueryEscape(opts.GitQuarantinePath),
		opts.ProtectedBranchID,
	)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return http.StatusInternalServerError, fmt.Sprintf("Unable to contact gitea: %v", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, decodeJSONError(resp).Err
	}

	return http.StatusOK, ""
}

// HookPostReceive updates services and users
func HookPostReceive(ownerName, repoName string, opts HookOptions) (map[string]interface{}, string) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/hook/post-receive/%s/%s?old=%s&new=%s&ref=%s&userID=%d&username=%s",
		url.PathEscape(ownerName),
		url.PathEscape(repoName),
		url.QueryEscape(opts.OldCommitID),
		url.QueryEscape(opts.NewCommitID),
		url.QueryEscape(opts.RefFullName),
		opts.UserID,
		url.QueryEscape(opts.UserName))

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return nil, fmt.Sprintf("Unable to contact gitea: %v", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeJSONError(resp).Err
	}
	res := map[string]interface{}{}
	_ = json.NewDecoder(resp.Body).Decode(&res)

	return res, ""
}
