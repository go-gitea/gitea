// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package i18n

import (
	"code.gitea.io/gitea/modules/util"
)

var (
	ErrLocaleAlreadyExist = util.SilentWrap{Message: "lang already exists", Err: util.ErrAlreadyExist}
	ErrUncertainArguments = util.SilentWrap{Message: "arguments to i18n should not contain uncertain slices", Err: util.ErrInvalidArgument}
)
