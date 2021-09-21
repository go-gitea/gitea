// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package login

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
	"xorm.io/xorm/schemas"
)

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
	assert.NoError(t, db.PrepareTestDatabase())

	loginSourceSchema, err := db.TableInfo(new(Source))
	assert.NoError(t, err)

	RegisterTypeConfig(OAuth2, new(TestSource))

	CreateSource(&Source{
		Type:     OAuth2,
		Name:     "TestSource",
		IsActive: false,
		Cfg: &TestSource{
			Provider: "ConvertibleSourceName",
			ClientID: "42",
		},
	})

	sb := new(strings.Builder)

	db.DumpTables([]*schemas.Table{loginSourceSchema}, sb)

	assert.Contains(t, sb.String(), `"Provider":"ConvertibleSourceName"`)
}
