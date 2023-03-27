// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
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

		data, err := yaml.Marshal(objs)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		if err != nil {
			return err
		}
		if len(objs) < bufferSize {
			break
		}
		start += len(objs)
	}

	return nil
}
