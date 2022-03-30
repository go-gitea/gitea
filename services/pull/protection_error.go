// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

type ErrIsClosed struct{}

func IsErrIsClosed(err error) bool {
	_, ok := err.(ErrIsClosed)
	return ok
}

func (err ErrIsClosed) Error() string {
	return "pull is cosed"
}

type ErrUserNotAllowedToMerge struct{}

func IsErrUserNotAllowedToMerge(err error) bool {
	_, ok := err.(ErrUserNotAllowedToMerge)
	return ok
}

func (err ErrUserNotAllowedToMerge) Error() string {
	return "user not allowed to merge"
}

type ErrHasMerged struct{}

func IsErrHasMerged(err error) bool {
	_, ok := err.(ErrHasMerged)
	return ok
}

func (err ErrHasMerged) Error() string {
	return "has already been merged"
}

type ErrIsWorkInProgress struct{}

func IsErrIsWorkInProgress(err error) bool {
	_, ok := err.(ErrIsWorkInProgress)
	return ok
}

func (err ErrIsWorkInProgress) Error() string {
	return "work in progress PRs cannot be merged"
}

type ErrNotMergableState struct{}

func IsErrNotMergableState(err error) bool {
	_, ok := err.(ErrNotMergableState)
	return ok
}

func (err ErrNotMergableState) Error() string {
	return "not in mergeable state"
}

type ErrDependenciesLeft struct{}

func IsErrDependenciesLeft(err error) bool {
	_, ok := err.(ErrDependenciesLeft)
	return ok
}

func (err ErrDependenciesLeft) Error() string {
	return "is blocked by an open dependency"
}
