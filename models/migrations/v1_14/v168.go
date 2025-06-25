// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14 //nolint:revive // version underscore

import "xorm.io/xorm"

func RecreateUserTableToFixDefaultValues(_ *xorm.Engine) error {
	return nil
}
