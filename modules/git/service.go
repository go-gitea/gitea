// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"code.gitea.io/gitea/modules/git/service"
	"code.gitea.io/gitea/modules/log"
)

var serviceProviders = map[string]service.GitService{}

// Service provides a reference to the selected GitService
var Service service.GitService

// RegisterService registers a providerService with a particular provider name
func RegisterService(name string, providerService service.GitService) {
	serviceProviders[name] = providerService
}

// SetServiceProvider sets the default Service
func SetServiceProvider(name string) {
	ok := false
	Service, ok = serviceProviders[name]
	if ok {
		return
	}
	log.Warn("Unknown git service provider %s reverting to default", name)

	Service = serviceProviders[""]
}
