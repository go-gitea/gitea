// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package saml

import "context"

// RegisterSource causes an OAuth2 configuration to be registered
func (source *Source) RegisterSource() error {
	samlRWMutex.Lock()
	defer samlRWMutex.Unlock()
	var err error
	source, err = createProvider(context.Background(), source)
	if err == nil {
		providers[source.authSource.Name] = *source
	}
	return err
}

// UnregisterSource causes an SAML configuration to be unregistered
func (source *Source) UnregisterSource() error {
	samlRWMutex.Lock()
	defer samlRWMutex.Unlock()
	delete(providers, source.authSource.Name)
	return nil
}
