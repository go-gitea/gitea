// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type ourConfig struct {
	name      string
	bufferLen int64
	subname   string
	provider  string
	config    string
}

// ConfigureFromFile configures logging from a provided file
func ConfigureFromFile(filename string) error {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return ConfigureFromBytes(bytes)
}

// ConfigureFromBytes configures logging from provided []byte
func ConfigureFromBytes(config []byte) error {
	configs := []ourConfig{}
	configMap := make(map[string]interface{})

	err := json.Unmarshal(config, &configMap)
	if err != nil {
		return err
	}

	defaultBufferLen, ok := configMap["DEFAULT_BUFFER_LEN"].(int64)
	if !ok || defaultBufferLen == 0 {
		defaultBufferLen = 1000
	}

	for name, loggerconfigInterface := range configMap {
		if name == "DEFAULT_BUFFER_LEN" {
			continue
		}
		loggerconfig, ok := loggerconfigInterface.(map[string]interface{})
		if !ok {
			return &ErrBadConfig{
				message: fmt.Sprintf("Bad configuration for %s", name),
			}
		}
		for subname, subloggerconfigInterface := range loggerconfig {
			subloggerconfig, ok := subloggerconfigInterface.(map[string]interface{})
			if !ok {
				return &ErrBadConfig{
					message: fmt.Sprintf("Bad configuration for %s:%s", name, subname),
				}
			}
			bufferLen, ok := loggerconfig["bufferLen"].(int64)
			if !ok || bufferLen == 0 {
				bufferLen = defaultBufferLen
			}
			provider, ok := subloggerconfig["provider"].(string)
			if !ok || provider == "" {
				provider = subname
			}
			subconfigInterface, ok := subloggerconfig["config"]

			subconfig, err := json.Marshal(subconfigInterface)
			if err != nil {
				return &ErrBadConfig{
					message: fmt.Sprintf("Bad configuration for %s:%s, provider %s: %v", name, subname, provider, err),
				}
			}
			configs = append(configs, ourConfig{
				name:      name,
				subname:   subname,
				bufferLen: bufferLen,
				provider:  provider,
				config:    string(subconfig),
			})
		}
	}
	if len(configs) == 0 {
		return &ErrBadConfig{
			message: fmt.Sprintf("Bad configuration. No loggers."),
		}
	}
	for _, c := range configs {
		err = NewNamedLogger(c.name, c.bufferLen, c.subname, c.provider, c.config)
		if err != nil {
			return err
		}
	}
	return nil
}
