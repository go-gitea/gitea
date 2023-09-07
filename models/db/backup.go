// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// BackupDatabaseAsFixtures backup all data from database to fixtures files on dirPath
func BackupDatabaseAsFixtures(dirPath string) error {
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return err
	}

	for _, t := range tables {
		if err := backupTableFixtures(t, dirPath); err != nil {
			return err
		}
	}
	return nil
}

func backupTableFixtures(bean interface{}, dirPath string) error {
	table, err := x.TableInfo(bean)
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dirPath, table.Name+".yml"))
	if err != nil {
		return err
	}
	defer f.Close()

	const bufferSize = 100
	start := 0
	for {
		objs, err := x.Table(table.Name).Limit(bufferSize, start).QueryInterface()
		if err != nil {
			return err
		}
		if len(objs) == 0 {
			break
		}

		for _, obj := range objs {
			for k, v := range obj {
				if vv, ok := v.([]byte); ok {
					obj[k] = string(vv)
				}
			}
			bs, err := yaml.Marshal([]any{obj})
			if err != nil {
				return err
			}
			if _, err := f.Write(bs); err != nil {
				return err
			}
			if _, err := f.Write([]byte{'\n'}); err != nil {
				return err
			}
		}

		if len(objs) < bufferSize {
			break
		}
		start += len(objs)
	}

	return nil
}
