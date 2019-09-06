// Copyright 2013 Martini Authors
// Copyright 2014 The Macaron Authors
// Copyright 2019 The Gitea Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package context

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"

	"gitea.com/macaron/macaron"
)

// Recovery returns a middleware that recovers from any panics and writes a 500 and a log if so.
// Although similar to macaron.Recovery() the main difference is that this error will be created
// with the gitea 500 page.
func Recovery() macaron.Handler {
	return func(ctx *Context) {
		defer func() {
			if err := recover(); err != nil {
				combinedErr := fmt.Errorf("%s\n%s", err, log.Stack(2))
				ctx.ServerError("PANIC:", combinedErr)
			}
		}()

		ctx.Next()
	}
}
