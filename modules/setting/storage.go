// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"path/filepath"
	"reflect"

	ini "gopkg.in/ini.v1"
)

// Storage represents configuration of storages
type Storage struct {
	Type        string
	Path        string
	Section     *ini.Section
	ServeDirect bool
}

// MapTo implements the Mappable interface
func (s *Storage) MapTo(v interface{}) error {
	pathValue := reflect.ValueOf(v).Elem().FieldByName("Path")
	if pathValue.IsValid() && pathValue.Kind() == reflect.String {
		pathValue.SetString(s.Path)
	}
	if s.Section != nil {
		return s.Section.MapTo(v)
	}
	return nil
}

func getStorage(name, typ string, overrides ...*ini.Section) Storage {
	const sectionName = "storage"
	sec := Cfg.Section(sectionName)

	if len(overrides) == 0 {
		overrides = []*ini.Section{
			Cfg.Section(sectionName + "." + typ),
			Cfg.Section(sectionName + "." + name),
		}
	}

	var storage Storage

	storage.Type = sec.Key("STORAGE_TYPE").MustString(typ)
	storage.ServeDirect = sec.Key("SERVE_DIRECT").MustBool(false)

	// Global Defaults
	sec.Key("MINIO_ENDPOINT").MustString("localhost:9000")
	sec.Key("MINIO_ACCESS_KEY_ID").MustString("")
	sec.Key("MINIO_SECRET_ACCESS_KEY").MustString("")
	sec.Key("MINIO_BUCKET").MustString("gitea")
	sec.Key("MINIO_LOCATION").MustString("us-east-1")
	sec.Key("MINIO_USE_SSL").MustBool(false)

	storage.Section = sec

	for _, override := range overrides {
		for _, key := range storage.Section.Keys() {
			if !override.HasKey(key.Name()) {
				_, _ = override.NewKey(key.Name(), key.Value())
			}
		}
		storage.ServeDirect = override.Key("SERVE_DIRECT").MustBool(false)
		storage.Section = override
	}

	// Specific defaults
	storage.Path = storage.Section.Key("PATH").MustString(filepath.Join(AppDataPath, name))
	if !filepath.IsAbs(storage.Path) {
		storage.Path = filepath.Join(AppWorkPath, storage.Path)
		storage.Section.Key("PATH").SetValue(storage.Path)
	}
	storage.Section.Key("MINIO_BASE_PATH").MustString(name + "/")

	return storage
}
