// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build !windows

package web

import auth_service "code.gitea.io/gitea/services/auth"

func specialAdd(group *auth_service.Group) {}
