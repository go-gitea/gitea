// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package password

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/gitea/modules/auth/password/pwn"
	"code.gitea.io/gitea/modules/setting"
)

var ErrIsPwned = errors.New("password has been pwned")

type ErrIsPwnedRequest struct {
	err error
}

func IsErrIsPwnedRequest(err error) bool {
	_, ok := err.(ErrIsPwnedRequest)
	return ok
}

func (err ErrIsPwnedRequest) Error() string {
	return fmt.Sprintf("using Have-I-Been-Pwned service failed: %v", err.err)
}

func (err ErrIsPwnedRequest) Unwrap() error {
	return err.err
}

// IsPwned checks whether a password has been pwned
// If a password has not been pwned, no error is returned.
func IsPwned(ctx context.Context, password string) error {
	if !setting.PasswordCheckPwn {
		return nil
	}

	client := pwn.New(pwn.WithContext(ctx))
	count, err := client.CheckPassword(password, true)
	if err != nil {
		return ErrIsPwnedRequest{err}
	}

	if count > 0 {
		return ErrIsPwned
	}

	return nil
}
