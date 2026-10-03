// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package deploykey

import (
	"fmt"

	"gitea.dev/modules/util"
)

// ErrDeployKeyNotExist represents a "DeployKeyNotExist" kind of error.
type ErrDeployKeyNotExist struct {
	ID     int64
	KeyID  int64
	RepoID int64
}

// IsErrDeployKeyNotExist checks if an error is a ErrDeployKeyNotExist.
func IsErrDeployKeyNotExist(err error) bool {
	_, ok := err.(ErrDeployKeyNotExist)
	return ok
}

func (err ErrDeployKeyNotExist) Error() string {
	return fmt.Sprintf("Deploy key does not exist [id: %d, key_id: %d, repo_id: %d]", err.ID, err.KeyID, err.RepoID)
}

func (err ErrDeployKeyNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrDeployKeyAlreadyExist represents a "DeployKeyAlreadyExist" kind of error.
type ErrDeployKeyAlreadyExist struct {
	KeyID  int64
	RepoID int64
}

// IsErrDeployKeyAlreadyExist checks if an error is a ErrDeployKeyAlreadyExist.
func IsErrDeployKeyAlreadyExist(err error) bool {
	_, ok := err.(ErrDeployKeyAlreadyExist)
	return ok
}

func (err ErrDeployKeyAlreadyExist) Error() string {
	return fmt.Sprintf("public key already exists [key_id: %d, repo_id: %d]", err.KeyID, err.RepoID)
}

func (err ErrDeployKeyAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrDeployKeyNameAlreadyUsed represents a "DeployKeyNameAlreadyUsed" kind of error.
type ErrDeployKeyNameAlreadyUsed struct {
	RepoID int64
	Name   string
}

// IsErrDeployKeyNameAlreadyUsed checks if an error is a ErrDeployKeyNameAlreadyUsed.
func IsErrDeployKeyNameAlreadyUsed(err error) bool {
	_, ok := err.(ErrDeployKeyNameAlreadyUsed)
	return ok
}

func (err ErrDeployKeyNameAlreadyUsed) Error() string {
	return fmt.Sprintf("public key with name already exists [repo_id: %d, name: %s]", err.RepoID, err.Name)
}

func (err ErrDeployKeyNameAlreadyUsed) Unwrap() error {
	return util.ErrNotExist
}
