// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"strings"
	"time"
)

// ErrExecTimeout error when exec timed out
type ErrExecTimeout struct {
	Duration time.Duration
}

// IsErrExecTimeout if some error is ErrExecTimeout
func IsErrExecTimeout(err error) bool {
	_, ok := err.(ErrExecTimeout)
	return ok
}

func (err ErrExecTimeout) Error() string {
	return fmt.Sprintf("execution is timeout [duration: %v]", err.Duration)
}

// ErrNotExist commit not exist error
type ErrNotExist struct {
	ID      string
	RelPath string
}

// IsErrNotExist if some error is ErrNotExist
func IsErrNotExist(err error) bool {
	_, ok := err.(ErrNotExist)
	return ok
}

func (err ErrNotExist) Error() string {
	return fmt.Sprintf("object does not exist [id: %s, rel_path: %s]", err.ID, err.RelPath)
}

// ErrBadLink entry.FollowLink error
type ErrBadLink struct {
	Name    string
	Message string
}

func (err ErrBadLink) Error() string {
	return fmt.Sprintf("%s: %s", err.Name, err.Message)
}

// IsErrBadLink if some error is ErrBadLink
func IsErrBadLink(err error) bool {
	_, ok := err.(ErrBadLink)
	return ok
}

// ErrUnsupportedVersion error when required git version not matched
type ErrUnsupportedVersion struct {
	Required string
}

// IsErrUnsupportedVersion if some error is ErrUnsupportedVersion
func IsErrUnsupportedVersion(err error) bool {
	_, ok := err.(ErrUnsupportedVersion)
	return ok
}

func (err ErrUnsupportedVersion) Error() string {
	return fmt.Sprintf("Operation requires higher version [required: %s]", err.Required)
}

// ErrBranchNotExist represents a "BranchNotExist" kind of error.
type ErrBranchNotExist struct {
	Name string
}

// IsErrBranchNotExist checks if an error is a ErrBranchNotExist.
func IsErrBranchNotExist(err error) bool {
	_, ok := err.(ErrBranchNotExist)
	return ok
}

func (err ErrBranchNotExist) Error() string {
	return fmt.Sprintf("branch does not exist [name: %s]", err.Name)
}

// ErrPushOutOfDate represents an error if merging fails due to unrelated histories
type ErrPushOutOfDate struct {
	StdOut string
	StdErr string
	Err    error
}

// IsErrPushOutOfDate checks if an error is a ErrPushOutOfDate.
func IsErrPushOutOfDate(err error) bool {
	_, ok := err.(*ErrPushOutOfDate)
	return ok
}

func (err *ErrPushOutOfDate) Error() string {
	return fmt.Sprintf("PushOutOfDate Error: %v: %s\n%s", err.Err, err.StdErr, err.StdOut)
}

// Unwrap unwraps the underlying error
func (err *ErrPushOutOfDate) Unwrap() error {
	return fmt.Errorf("%v - %s", err.Err, err.StdErr)
}

// ErrPushRejected represents an error if merging fails due to rejection from a hook
type ErrPushRejected struct {
	Message string
	StdOut  string
	StdErr  string
	Err     error
}

// IsErrPushRejected checks if an error is a ErrPushRejected.
func IsErrPushRejected(err error) bool {
	_, ok := err.(*ErrPushRejected)
	return ok
}

func (err *ErrPushRejected) Error() string {
	return fmt.Sprintf("PushRejected Error: %v: %s\n%s", err.Err, err.StdErr, err.StdOut)
}

// Unwrap unwraps the underlying error
func (err *ErrPushRejected) Unwrap() error {
	return fmt.Errorf("%v - %s", err.Err, err.StdErr)
}

// GenerateMessage generates the remote message from the stderr
func (err *ErrPushRejected) GenerateMessage() {
	messageBuilder := &strings.Builder{}
	i := strings.Index(err.StdErr, "remote: ")
	if i < 0 {
		err.Message = ""
		return
	}
	for {
		if len(err.StdErr) <= i+8 {
			break
		}
		if err.StdErr[i:i+8] != "remote: " {
			break
		}
		i += 8
		nl := strings.IndexByte(err.StdErr[i:], '\n')
		if nl >= 0 {
			messageBuilder.WriteString(err.StdErr[i : i+nl+1])
			i = i + nl + 1
		} else {
			messageBuilder.WriteString(err.StdErr[i:])
			i = len(err.StdErr)
		}
	}
	err.Message = strings.TrimSpace(messageBuilder.String())
}

// ErrMoreThanOne represents an error if pull request fails when there are more than one sources (branch, tag) with the same name
type ErrMoreThanOne struct {
	StdOut string
	StdErr string
	Err    error
}

// IsErrMoreThanOne checks if an error is a ErrMoreThanOne
func IsErrMoreThanOne(err error) bool {
	_, ok := err.(*ErrMoreThanOne)
	return ok
}

func (err *ErrMoreThanOne) Error() string {
	return fmt.Sprintf("ErrMoreThanOne Error: %v: %s\n%s", err.Err, err.StdErr, err.StdOut)
}

// ErrRefNotFound represents a "RefDoesMotExist" kind of error.
type ErrRefNotFound struct {
	RefName string
}

// IsErrRefNotFound checks if an error is a ErrRefNotFound.
func IsErrRefNotFound(err error) bool {
	_, ok := err.(ErrRefNotFound)
	return ok
}

func (err ErrRefNotFound) Error() string {
	return fmt.Sprintf("ref does not exist [ref_name: %s]", err.RefName)
}

// ErrInvalidRefName represents a "InvalidRefName" kind of error.
type ErrInvalidRefName struct {
	RefName string
	Reason  string
}

// IsErrInvalidRefName checks if an error is a ErrInvalidRefName.
func IsErrInvalidRefName(err error) bool {
	_, ok := err.(ErrInvalidRefName)
	return ok
}

func (err ErrInvalidRefName) Error() string {
	return fmt.Sprintf("ref name is not valid: %s [ref_name: %s]", err.Reason, err.RefName)
}

// ErrProtectedRefName represents a "ProtectedRefName" kind of error.
type ErrProtectedRefName struct {
	RefName string
	Message string
}

// IsErrProtectedRefName checks if an error is a ErrProtectedRefName.
func IsErrProtectedRefName(err error) bool {
	_, ok := err.(ErrProtectedRefName)
	return ok
}

func (err ErrProtectedRefName) Error() string {
	str := fmt.Sprintf("ref name is protected [ref_name: %s]", err.RefName)
	if err.Message != "" {
		str = fmt.Sprintf("%s: %s", str, err.Message)
	}
	return str
}

// ErrRefAlreadyExists represents an error that ref with such name already exists.
type ErrRefAlreadyExists struct {
	RefName string
}

// IsErrRefAlreadyExists checks if an error is an ErrRefAlreadyExists.
func IsErrRefAlreadyExists(err error) bool {
	_, ok := err.(ErrRefAlreadyExists)
	return ok
}

func (err ErrRefAlreadyExists) Error() string {
	return fmt.Sprintf("ref already exists [name: %s]", err.RefName)
}
