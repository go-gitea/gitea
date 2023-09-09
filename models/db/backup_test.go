// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestBackupRestore(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	d, err := os.MkdirTemp(os.TempDir(), "backup_restore")
	assert.NoError(t, err)

	assert.NoError(t, db.BackupDatabaseAsFixtures(d))

	f, err := os.Open(d)
	assert.NoError(t, err)
	defer f.Close()

	entries, err := f.ReadDir(0)
	assert.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileEqual(t, filepath.Join("..", "fixtures", entry.Name()), filepath.Join(d, entry.Name()))
	}

	// assert.NoError(t, db.RestoreDatabase(d))
}

func sortTable(tablename string, data []map[string]any) {
	sort.Slice(data, func(i, j int) bool {
		if tablename == "issue_index" {
			return data[i]["group_id"].(int) < data[j]["group_id"].(int)
		}
		if tablename == "repo_topic" {
			return data[i]["repo_id"].(int) < data[j]["repo_id"].(int)
		}
		return data[i]["id"].(int) < data[j]["id"].(int)
	})
}

func convertBool(b any) bool {
	switch rr := b.(type) {
	case bool:
		return rr
	case int:
		return rr != 0
	default:
		r, _ := strconv.ParseBool(b.(string))
		return r
	}
}

func fileEqual(t *testing.T, a, b string) {
	filename := filepath.Base(a)
	tablename := filename[:len(filename)-len(filepath.Ext(filename))]
	t.Run(filename, func(t *testing.T) {
		bs1, err := os.ReadFile(a)
		assert.NoError(t, err)

		var data1 []map[string]any
		assert.NoError(t, yaml.Unmarshal(bs1, &data1))

		sortTable(tablename, data1)

		bs2, err := os.ReadFile(b)
		assert.NoError(t, err)

		var data2 []map[string]any
		assert.NoError(t, yaml.Unmarshal(bs2, &data2))

		sortTable(tablename, data2)

		assert.EqualValues(t, len(data1), len(data2), fmt.Sprintf("compare %s with %s", a, b))
		for i := range data1 {
			assert.LessOrEqual(t, len(data1[i]), len(data2[i]), fmt.Sprintf("compare %s with %s", a, b))
			for k, v := range data1[i] {
				switch vv := v.(type) {
				case bool:
					assert.EqualValues(t, vv, convertBool(data2[i][k]), fmt.Sprintf("compare %s with %s", a, b))
				case nil:
					switch data2[i][k].(type) {
					case nil:
					case string:
						assert.Empty(t, data2[i][k])
					default:
						panic(fmt.Sprintf("%#v", data2[i][k]))
					}
				default:
					assert.EqualValues(t, v, data2[i][k], fmt.Sprintf("compare %#v with %#v", v, data2[i][k]))
				}
			}
		}
	})
}
