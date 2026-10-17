// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package types

import "net/http"

// PreMiddlewareProvider is a special middleware provider which will be executed
// before other middlewares on the same "routing" level (AfterRouting/Group/Methods/Any, but not BeforeRouting).
// A route can do something (e.g.: set middleware options) at the place where it is declared,
// and the code will be executed before other middlewares which are added before the declaration.
// Use cases: mark a route with some meta info, set some options for middlewares, etc.
type PreMiddlewareProvider func(next http.Handler) http.Handler
