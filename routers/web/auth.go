// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package web

import auth_service "code.gitea.io/gitea/services/auth"

func specialAdd(group *auth_service.Group) {}
