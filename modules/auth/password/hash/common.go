// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hash

import (
	"strconv"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

func parseIntParam(value, param, algorithmName, config string, previousErr error) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Error("invalid integer for %s representation in %s hash spec %s", param, algorithmName, config)
		return 0, err
	}
	return parsed, previousErr // <- Keep the previous error as this function should still return an error once everything has been checked if any call failed
}

func parseUintParam[T uint32 | uint8](value, param, algorithmName, config string, previousErr error) (ret T, _ error) {
	_, isUint32 := any(ret).(uint32)
	parsed, err := strconv.ParseUint(value, 10, util.Iif(isUint32, 32, 8))
	if err != nil {
		log.Error("invalid integer for %s representation in %s hash spec %s", param, algorithmName, config)
		return 0, err
	}
	return T(parsed), previousErr // <- Keep the previous error as this function should still return an error once everything has been checked if any call failed
}
