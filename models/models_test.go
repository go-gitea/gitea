// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"xorm.io/xorm/schemas"

	"github.com/stretchr/testify/assert"
)

func TestDumpDatabase(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	dir, err := ioutil.TempDir(os.TempDir(), "dump")
	assert.NoError(t, err)

	type Version struct {
		ID      int64 `xorm:"pk autoincr"`
		Version int64
	}
	assert.NoError(t, x.Sync2(new(Version)))

	for _, dbName := range setting.SupportedDatabases {
		dbType := setting.GetDBTypeByName(dbName)
		assert.NoError(t, DumpDatabase(filepath.Join(dir, dbType+".sql"), dbType))
	}
}

type TestSource struct {
	Provider                      string
	ClientID                      string
	ClientSecret                  string
	OpenIDConnectAutoDiscoveryURL string
	IconURL                       string
}

// FromDB fills up a LDAPConfig from serialized format.
func (source *TestSource) FromDB(bs []byte) error {
	return json.Unmarshal(bs, &source)
}

// ToDB exports a LDAPConfig to a serialized format.
func (source *TestSource) ToDB() ([]byte, error) {
	return json.Marshal(source)
}

func TestDumpLoginSource(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	loginSourceSchema, err := x.TableInfo(new(LoginSource))
	assert.NoError(t, err)

	RegisterLoginTypeConfig(LoginOAuth2, new(TestSource))

	CreateLoginSource(&LoginSource{
		Type:     LoginOAuth2,
		Name:     "TestSource",
		IsActive: false,
		Cfg: &TestSource{
			Provider: "ConvertibleSourceName",
			ClientID: "42",
		},
	})

	sb := new(strings.Builder)

	x.DumpTables([]*schemas.Table{loginSourceSchema}, sb)

	assert.Contains(t, sb.String(), `"Provider":"ConvertibleSourceName"`)
}
