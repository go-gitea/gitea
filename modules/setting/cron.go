// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"reflect"

	"code.gitea.io/gitea/modules/setting/base"
)

// GetCronSettings maps the cron subsection to the provided config
func GetCronSettings(name string, config interface{}) (interface{}, error) {
	return getCronSettings(CfgProvider, name, config)
}

func getCronSettings(rootCfg base.ConfigProvider, name string, config interface{}) (interface{}, error) {
	if err := rootCfg.Section("cron." + name).MapTo(config); err != nil {
		return config, err
	}

	typ := reflect.TypeOf(config).Elem()
	val := reflect.ValueOf(config).Elem()

	for i := 0; i < typ.NumField(); i++ {
		field := val.Field(i)
		tpField := typ.Field(i)
		if tpField.Type.Kind() == reflect.Struct && tpField.Anonymous {
			if err := rootCfg.Section("cron." + name).MapTo(field.Addr().Interface()); err != nil {
				return config, err
			}
		}
	}

	return config, nil
}
