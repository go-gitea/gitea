// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/perm"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
)

// KeyAndOwner is the response from ServNoCommand
type KeyAndOwner struct {
	Key   *asymkey_model.PublicKey `json:"key"`
	Owner *user_model.User         `json:"user"`
}

// ServNoCommand returns information about the provided key
func ServNoCommand(ctx context.Context, keyID int64) (*asymkey_model.PublicKey, *user_model.User, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/serv/none/%d",
		keyID)
	resp, err := newInternalRequest(ctx, reqURL, "GET").Response()
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("%s", decodeJSONError(resp).Err)
	}

	var keyAndOwner KeyAndOwner
	if err := json.NewDecoder(resp.Body).Decode(&keyAndOwner); err != nil {
		return nil, nil, err
	}
	return keyAndOwner.Key, keyAndOwner.Owner, nil
}

// ServCommandResults are the results of a call to the private route serv
type ServCommandResults struct {
	IsWiki      bool
	DeployKeyID int64
	KeyID       int64  // public key
	KeyName     string // this field is ambiguous, it can be the name of DeployKey, or the name of the PublicKey
	UserName    string
	UserEmail   string
	UserID      int64
	OwnerName   string
	RepoName    string
	RepoID      int64
}

// ErrServCommand is an error returned from ServCommmand.
type ErrServCommand struct {
	Results    ServCommandResults
	Err        string
	StatusCode int
}

func (err ErrServCommand) Error() string {
	return err.Err
}

// IsErrServCommand checks if an error is a ErrServCommand.
func IsErrServCommand(err error) bool {
	_, ok := err.(ErrServCommand)
	return ok
}

// ServCommand preps for a serv call
func ServCommand(ctx context.Context, keyID int64, ownerName, repoName string, mode perm.AccessMode, verbs ...string) (*ServCommandResults, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/serv/command/%d/%s/%s?mode=%d",
		keyID,
		url.PathEscape(ownerName),
		url.PathEscape(repoName),
		mode)
	for _, verb := range verbs {
		if verb != "" {
			reqURL += fmt.Sprintf("&verb=%s", url.QueryEscape(verb))
		}
	}

	resp, err := newInternalRequest(ctx, reqURL, "GET").Response()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errServCommand ErrServCommand
		if err := json.NewDecoder(resp.Body).Decode(&errServCommand); err != nil {
			return nil, err
		}
		errServCommand.StatusCode = resp.StatusCode
		return nil, errServCommand
	}
	var results ServCommandResults
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}
	return &results, nil

}
