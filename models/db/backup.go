// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"xorm.io/xorm"
)

// BackupDatabaseAsFixtures backup all data from database to fixtures files on dirPath
func BackupDatabaseAsFixtures(dirPath string) error {
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return err
	}

	for _, t := range tables {
		if err := backupTableFixtures(x, t, dirPath); err != nil {
			return err
		}
	}
	return nil
}

func toNode(tableName, col string, v interface{}) *yaml.Node {
	if v == nil {
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: "",
		}
	}
	switch vv := v.(type) {
	case string:
		if tableName == "action_task" && col == "log_indexes" {
			return &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!binary",
				Value: base64.StdEncoding.EncodeToString([]byte(vv)),
			}
		}
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: vv,
		}
	case []byte:
		if tableName == "action_task" && col == "log_indexes" {
			return &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!binary",
				Value: base64.StdEncoding.EncodeToString(vv),
			}
		}
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: string(vv),
		}
	case int, int64, int32, int8, int16, uint, uint64:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!int",
			Value: fmt.Sprintf("%d", vv),
		}
	case float64, float32:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!float",
			Value: fmt.Sprintf("%f", vv),
		}
	default:
		panic(fmt.Sprintf("unknow type %#v", v))
	}
}

func backupTableFixtures(e *xorm.Engine, bean interface{}, dirPath string) error {
	table, err := e.TableInfo(bean)
	if err != nil {
		return err
	}
	if isEmpty, err := e.IsTableEmpty(table.Name); err != nil {
		return err
	} else if isEmpty {
		return nil
	}

	f, err := os.Create(filepath.Join(dirPath, table.Name+".yml"))
	if err != nil {
		return err
	}
	defer f.Close()

	const bufferSize = 100
	start := 0

	for {
		objs, err := e.Table(table.Name).Limit(bufferSize, start).QueryInterface()
		if err != nil {
			return err
		}
		if len(objs) == 0 {
			break
		}

		for _, obj := range objs {
			node := yaml.Node{
				Kind: yaml.MappingNode,
			}
			for _, col := range table.ColumnsSeq() {
				v, ok := obj[col]
				if !ok {
					return fmt.Errorf("column %s has no value from database", col)
				}

				node.Content = append(node.Content,
					&yaml.Node{
						Kind:  yaml.ScalarNode,
						Tag:   "!!str",
						Value: col,
					},
					toNode(table.Name, col, v),
				)
			}

			bs, err := yaml.Marshal([]*yaml.Node{&node}) // with []any{} to ensure generated a list
			if err != nil {
				return fmt.Errorf("marshal table %s record %#v %#v failed: %v", table.Name, obj, node.Content[1], err)
			}
			if _, err := f.Write(bs); err != nil {
				return err
			}
			if _, err := f.Write([]byte{'\n'}); err != nil { // generate a blank line for human readable
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
