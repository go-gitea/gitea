// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

// GetCronSettings maps the cron subsection to the provided config
func GetCronSettings(name string, config interface{}) (interface{}, error) {
	err := Cfg.Section("cron." + name).MapTo(config)
	return config, err
}
