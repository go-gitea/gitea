// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"

	"gitea.dev/modules/setting"

	"github.com/lib/pq"
	"xorm.io/xorm/dialects"
)

type postgresSchemaDriver struct{}

var registerPostgresSchemaDriver = sync.OnceFunc(func() {
	sql.Register(sqlDriverPostgresSchema, &postgresSchemaDriver{})
	dialects.RegisterDriver(sqlDriverPostgresSchema, dialects.QueryDriver("postgres"))
})

// Open opens the postgres connection in the default manner with default schema support.
// It immediately runs "set_config" to set the search_path appropriately.
func (*postgresSchemaDriver) Open(connStr string) (driver.Conn, error) {
	conn, err := pq.Driver{}.Open(connStr)
	if err != nil {
		return nil, err
	}

	connExec, ok := conn.(driver.ExecerContext)
	if !ok {
		return nil, errors.New("postgres driver does not implement ExecerContext interface")
	}
	_, err = connExec.ExecContext(context.Background(), `SELECT set_config(
			'search_path',
			$1 || ',' || current_setting('search_path'),
			false)`,
		[]driver.NamedValue{{Ordinal: 1, Value: setting.Database.Schema}},
	)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}
