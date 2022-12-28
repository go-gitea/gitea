// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"fmt"

	"code.gitea.io/gitea/modules/util"
)

// ErrWebhookNotExist represents a "WebhookNotExist" kind of error.
type ErrWebhookNotExist struct {
	ID int64
}

// IsErrWebhookNotExist checks if an error is a ErrWebhookNotExist.
func IsErrWebhookNotExist(err error) bool {
	_, ok := err.(ErrWebhookNotExist)
	return ok
}

func (err ErrWebhookNotExist) Error() string {
	return fmt.Sprintf("webhook does not exist [id: %d]", err.ID)
}

func (err ErrWebhookNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrHookTaskNotExist represents a "HookTaskNotExist" kind of error.
type ErrHookTaskNotExist struct {
	TaskID int64
	HookID int64
	UUID   string
}

// IsErrWebhookNotExist checks if an error is a ErrWebhookNotExist.
func IsErrHookTaskNotExist(err error) bool {
	_, ok := err.(ErrHookTaskNotExist)
	return ok
}

func (err ErrHookTaskNotExist) Error() string {
	return fmt.Sprintf("hook task does not exist [task: %d, hook: %d, uuid: %s]", err.TaskID, err.HookID, err.UUID)
}

func (err ErrHookTaskNotExist) Unwrap() error {
	return util.ErrNotExist
}
