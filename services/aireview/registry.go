// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"fmt"
	"sync"
)

var (
	providersMu sync.RWMutex
	providers   = make(map[string]ProviderFactory)
)

// ProviderFactory creates a Provider instance.
type ProviderFactory func() Provider

// RegisterProvider registers a provider factory by name.
func RegisterProvider(name string, factory ProviderFactory) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers[name] = factory
}

// GetProvider returns a provider instance by name.
func GetProvider(name string) (Provider, error) {
	providersMu.RLock()
	factory, ok := providers[name]
	providersMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("aireview: unknown provider %q", name)
	}
	return factory(), nil
}

// HasProvider checks if a provider is registered.
func HasProvider(name string) bool {
	providersMu.RLock()
	defer providersMu.RUnlock()
	_, ok := providers[name]
	return ok
}

func init() {
	RegisterProvider("openrouter", func() Provider { return NewOpenAIProvider() })
	RegisterProvider("openai", func() Provider { return NewOpenAIProvider() })
}
