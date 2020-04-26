// Go MySQL Driver - A MySQL-Driver for Go's database/sql package
//
// Copyright 2013 The Go-MySQL-Driver Authors. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

// +build appengine

package mysql

import (
"code.gitea.io/gitea/traceinit"


	"google.golang.org/appengine/cloudsql"
)

func init () {
traceinit.Trace("vendor/github.com/go-sql-driver/mysql/appengine.go")




	RegisterDial("cloudsql", cloudsql.Dial)
}
