// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"fmt"

	"code.gitea.io/gitea/modules/util"
)

// ErrKeyUnableVerify represents a "KeyUnableVerify" kind of error.
type ErrKeyUnableVerify struct {
	Result string
}

// IsErrKeyUnableVerify checks if an error is a ErrKeyUnableVerify.
func IsErrKeyUnableVerify(err error) bool {
	_, ok := err.(ErrKeyUnableVerify)
	return ok
}

func (err ErrKeyUnableVerify) Error() string {
	return fmt.Sprintf("Unable to verify key content [result: %s]", err.Result)
}

// ErrKeyIsPrivate is returned when the provided key is a private key not a public key
var ErrKeyIsPrivate = util.ErrorWrap(util.ErrInvalidArgument, "the provided key is a private key")

// ErrKeyNotExist represents a "KeyNotExist" kind of error.
type ErrKeyNotExist struct {
	ID int64
}

// IsErrKeyNotExist checks if an error is a ErrKeyNotExist.
func IsErrKeyNotExist(err error) bool {
	_, ok := err.(ErrKeyNotExist)
	return ok
}

func (err ErrKeyNotExist) Error() string {
	return fmt.Sprintf("public key does not exist [id: %d]", err.ID)
}

func (err ErrKeyNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrKeyAlreadyExist represents a "KeyAlreadyExist" kind of error.
type ErrKeyAlreadyExist struct {
	OwnerID     int64
	Fingerprint string
	Content     string
}

// IsErrKeyAlreadyExist checks if an error is a ErrKeyAlreadyExist.
func IsErrKeyAlreadyExist(err error) bool {
	_, ok := err.(ErrKeyAlreadyExist)
	return ok
}

func (err ErrKeyAlreadyExist) Error() string {
	return fmt.Sprintf("public key already exists [owner_id: %d, finger_print: %s, content: %s]",
		err.OwnerID, err.Fingerprint, err.Content)
}

func (err ErrKeyAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrKeyNameAlreadyUsed represents a "KeyNameAlreadyUsed" kind of error.
type ErrKeyNameAlreadyUsed struct {
	OwnerID int64
	Name    string
}

// IsErrKeyNameAlreadyUsed checks if an error is a ErrKeyNameAlreadyUsed.
func IsErrKeyNameAlreadyUsed(err error) bool {
	_, ok := err.(ErrKeyNameAlreadyUsed)
	return ok
}

func (err ErrKeyNameAlreadyUsed) Error() string {
	return fmt.Sprintf("public key already exists [owner_id: %d, name: %s]", err.OwnerID, err.Name)
}

func (err ErrKeyNameAlreadyUsed) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrGPGNoEmailFound represents a "ErrGPGNoEmailFound" kind of error.
type ErrGPGNoEmailFound struct {
	FailedEmails []string
	ID           string
}

// IsErrGPGNoEmailFound checks if an error is a ErrGPGNoEmailFound.
func IsErrGPGNoEmailFound(err error) bool {
	_, ok := err.(ErrGPGNoEmailFound)
	return ok
}

func (err ErrGPGNoEmailFound) Error() string {
	return fmt.Sprintf("none of the emails attached to the GPG key could be found: %v", err.FailedEmails)
}

// ErrGPGInvalidTokenSignature represents a "ErrGPGInvalidTokenSignature" kind of error.
type ErrGPGInvalidTokenSignature struct {
	Wrapped error
	ID      string
}

// IsErrGPGInvalidTokenSignature checks if an error is a ErrGPGInvalidTokenSignature.
func IsErrGPGInvalidTokenSignature(err error) bool {
	_, ok := err.(ErrGPGInvalidTokenSignature)
	return ok
}

func (err ErrGPGInvalidTokenSignature) Error() string {
	return "the provided signature does not sign the token with the provided key"
}

// ErrGPGKeyParsing represents a "ErrGPGKeyParsing" kind of error.
type ErrGPGKeyParsing struct {
	ParseError error
}

// IsErrGPGKeyParsing checks if an error is a ErrGPGKeyParsing.
func IsErrGPGKeyParsing(err error) bool {
	_, ok := err.(ErrGPGKeyParsing)
	return ok
}

func (err ErrGPGKeyParsing) Error() string {
	return fmt.Sprintf("failed to parse gpg key %s", err.ParseError.Error())
}

// ErrGPGKeyNotExist represents a "GPGKeyNotExist" kind of error.
type ErrGPGKeyNotExist struct {
	ID int64
}

// IsErrGPGKeyNotExist checks if an error is a ErrGPGKeyNotExist.
func IsErrGPGKeyNotExist(err error) bool {
	_, ok := err.(ErrGPGKeyNotExist)
	return ok
}

func (err ErrGPGKeyNotExist) Error() string {
	return fmt.Sprintf("public gpg key does not exist [id: %d]", err.ID)
}

func (err ErrGPGKeyNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrGPGKeyImportNotExist represents a "GPGKeyImportNotExist" kind of error.
type ErrGPGKeyImportNotExist struct {
	ID string
}

// IsErrGPGKeyImportNotExist checks if an error is a ErrGPGKeyImportNotExist.
func IsErrGPGKeyImportNotExist(err error) bool {
	_, ok := err.(ErrGPGKeyImportNotExist)
	return ok
}

func (err ErrGPGKeyImportNotExist) Error() string {
	return fmt.Sprintf("public gpg key import does not exist [id: %s]", err.ID)
}

func (err ErrGPGKeyImportNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrGPGKeyIDAlreadyUsed represents a "GPGKeyIDAlreadyUsed" kind of error.
type ErrGPGKeyIDAlreadyUsed struct {
	KeyID string
}

// IsErrGPGKeyIDAlreadyUsed checks if an error is a ErrKeyNameAlreadyUsed.
func IsErrGPGKeyIDAlreadyUsed(err error) bool {
	_, ok := err.(ErrGPGKeyIDAlreadyUsed)
	return ok
}

func (err ErrGPGKeyIDAlreadyUsed) Error() string {
	return fmt.Sprintf("public key already exists [key_id: %s]", err.KeyID)
}

func (err ErrGPGKeyIDAlreadyUsed) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrGPGKeyAccessDenied represents a "GPGKeyAccessDenied" kind of Error.
type ErrGPGKeyAccessDenied struct {
	UserID int64
	KeyID  int64
}

// IsErrGPGKeyAccessDenied checks if an error is a ErrGPGKeyAccessDenied.
func IsErrGPGKeyAccessDenied(err error) bool {
	_, ok := err.(ErrGPGKeyAccessDenied)
	return ok
}

// Error pretty-prints an error of type ErrGPGKeyAccessDenied.
func (err ErrGPGKeyAccessDenied) Error() string {
	return fmt.Sprintf("user does not have access to the key [user_id: %d, key_id: %d]",
		err.UserID, err.KeyID)
}

func (err ErrGPGKeyAccessDenied) Unwrap() error {
	return util.ErrPermissionDenied
}

// ErrKeyAccessDenied represents a "KeyAccessDenied" kind of error.
type ErrKeyAccessDenied struct {
	UserID int64
	RepoID int64
	KeyID  int64
	Note   string
}

// IsErrKeyAccessDenied checks if an error is a ErrKeyAccessDenied.
func IsErrKeyAccessDenied(err error) bool {
	_, ok := err.(ErrKeyAccessDenied)
	return ok
}

func (err ErrKeyAccessDenied) Error() string {
	return fmt.Sprintf("user does not have access to the key [user_id: %d, repo_id: %d, key_id: %d, note: %s]",
		err.UserID, err.RepoID, err.KeyID, err.Note)
}

func (err ErrKeyAccessDenied) Unwrap() error {
	return util.ErrPermissionDenied
}

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

// ErrSSHInvalidTokenSignature represents a "ErrSSHInvalidTokenSignature" kind of error.
type ErrSSHInvalidTokenSignature struct {
	Wrapped     error
	Fingerprint string
}

// IsErrSSHInvalidTokenSignature checks if an error is a ErrSSHInvalidTokenSignature.
func IsErrSSHInvalidTokenSignature(err error) bool {
	_, ok := err.(ErrSSHInvalidTokenSignature)
	return ok
}

func (err ErrSSHInvalidTokenSignature) Error() string {
	return "the provided signature does not sign the token with the provided key"
}

func (err ErrSSHInvalidTokenSignature) Unwrap() error {
	return util.ErrInvalidArgument
}
