// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package i18n

import "errors"

var (
	ErrLocaleAlreadyExist = errors.New("lang already exists")
	ErrUncertainArguments = errors.New("arguments to i18n should not contain uncertain slices")
)
