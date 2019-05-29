// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

// LoggerProvider represents behaviors of a logger provider.
type LoggerProvider interface {
	Init(config string) error
	EventLogger
}

type loggerProvider func() LoggerProvider

var providers = make(map[string]loggerProvider)

// Register registers given logger provider to providers.
func Register(name string, log loggerProvider) {
	if log == nil {
		panic("log: register provider is nil")
	}
	if _, dup := providers[name]; dup {
		panic("log: register called twice for provider \"" + name + "\"")
	}
	providers[name] = log
}
