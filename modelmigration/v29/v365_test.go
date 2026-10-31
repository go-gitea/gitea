// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeLegacyConfigJSONValue(t *testing.T) {
	const (
		config = `{"Host":"smtp.example.com"}`
		packed = "≻潈瑳㨢猢瑭⹰硥浡汰⹥潣≭⁽"
	)
	for _, tc := range []struct {
		name, input, want string
	}{
		{"empty", "", ""},
		{"valid", config + " ", config + " "},
		{"unicode and large number", ` {"name":"例😀","id":9007199254740993} `, ` {"name":"例😀","id":9007199254740993} `},
		{"double encoded", packed, config},
		{"double encoded with BOM", "\ufeff" + packed, config},
		{"BOM", "\xff\xfe" + config, config},
		{"BOM with non-BMP rune", "\xff\xfe" + `{"name":"😀"}`, `{"name":"😀"}`},
		{"corrupt", "invalid", "invalid"},
		{"corrupt non-BMP rune", "invalid😀", "invalid😀"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeLegacyConfigJSON(tc.input)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, got, normalizeLegacyConfigJSON(got))
		})
	}
}

func TestNormalizeLegacyConfigJSON(t *testing.T) {
	type RepoUnit struct {
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64
		Config string `xorm:"TEXT"`
	}
	type LoginSource struct {
		ID   int64 `xorm:"pk autoincr"`
		Name string
		Cfg  string `xorm:"TEXT"`
	}
	x, cleanup := migrationtest.PrepareTestEnv(t, 0, new(RepoUnit), new(LoginSource))
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}
	defer test.MockVariableValue(&setting.Database.IterateBufferSize, 2)()

	inputs := []string{
		` {"id":9007199254740993,"name":"例😀"} `,
		"≻潈瑳㨢猢瑭⹰硥浡汰⹥潣≭⁽",
		"\ufeff≻潈瑳㨢猢瑭⹰硥浡汰⹥潣≭⁽",
		"",
		"invalid",
	}
	want := []string{inputs[0], `{"Host":"smtp.example.com"}`, `{"Host":"smtp.example.com"}`, "", "invalid", ""}
	for _, input := range inputs {
		_, err := x.Insert(&RepoUnit{RepoID: 42, Config: input})
		require.NoError(t, err)
		_, err = x.Insert(&LoginSource{Name: "test", Cfg: input})
		require.NoError(t, err)
	}
	_, err := x.Exec("INSERT INTO `repo_unit` (`repo_id`, `config`) VALUES (?, NULL)", 42)
	require.NoError(t, err)
	_, err = x.Exec("INSERT INTO `login_source` (`name`, `cfg`) VALUES (?, NULL)", "test")
	require.NoError(t, err)

	for range 2 {
		require.NoError(t, NormalizeLegacyConfigJSON(t.Context(), x))
		var units []RepoUnit
		require.NoError(t, x.Asc("id").Find(&units))
		require.Len(t, units, len(want))
		var sources []LoginSource
		require.NoError(t, x.Asc("id").Find(&sources))
		require.Len(t, sources, len(want))
		for i, config := range want {
			assert.Equal(t, config, units[i].Config)
			assert.EqualValues(t, 42, units[i].RepoID)
			assert.Equal(t, config, sources[i].Cfg)
			assert.Equal(t, "test", sources[i].Name)
		}
	}
}
