// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tidb

import (
	"errors"
	"net/url"
	"path/filepath"

	"github.com/go-xorm/core"
)

var (
	_ core.Dialect = (*tidb)(nil)

	DBType core.DbType = "tidb"
)

func init() {
	core.RegisterDriver(string(DBType), &tidbDriver{})
	core.RegisterDialect(DBType, func() core.Dialect {
		return &tidb{}
	})
}

type tidbDriver struct {
}

func (p *tidbDriver) Parse(driverName, dataSourceName string) (*core.Uri, error) {
	u, err := url.Parse(dataSourceName)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "goleveldb" && u.Scheme != "memory" && u.Scheme != "boltdb" {
		return nil, errors.New(u.Scheme + " is not supported yet.")
	}
	path := filepath.Join(u.Host, u.Path)
	dbName := filepath.Clean(filepath.Base(path))

	uri := &core.Uri{
		DbType: DBType,
		DbName: dbName,
	}

	return uri, nil
}
