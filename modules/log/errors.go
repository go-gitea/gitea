// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import "fmt"

// ErrTimeout represents a "Timeout" kind of error.
type ErrTimeout struct {
	Name     string
	Provider string
}

// IsErrTimeout checks if an error is a ErrTimeout.
func IsErrTimeout(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(ErrTimeout)
	return ok
}

func (err ErrTimeout) Error() string {
	return fmt.Sprintf("Log Timeout for %s (%s)", err.Name, err.Provider)
}

// ErrUnknownProvider represents a "Unknown Provider" kind of error.
type ErrUnknownProvider struct {
	Provider string
}

// IsErrUnknownProvider checks if an error is a ErrUnknownProvider.
func IsErrUnknownProvider(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(ErrUnknownProvider)
	return ok
}

func (err ErrUnknownProvider) Error() string {
	return fmt.Sprintf("Unknown Log Provider \"%s\" (Was it registered?)", err.Provider)
}

// ErrDuplicateName represents a Duplicate Name error
type ErrDuplicateName struct {
	Name string
}

// IsErrDuplicateName checks if an error is a ErrDuplicateName.
func IsErrDuplicateName(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(ErrDuplicateName)
	return ok
}

func (err ErrDuplicateName) Error() string {
	return fmt.Sprintf("Duplicate named logger: %s", err.Name)
}
