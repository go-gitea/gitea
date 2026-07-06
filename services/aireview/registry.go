// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"fmt"
	"strings"
	"sync"

	"gitea.dev/modules/setting"
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
// If name is "auto", it auto-detects the provider from the configured API URL.
func GetProvider(name string) (Provider, error) {
	if name == "auto" {
		name = DetectProviderFromURL(setting.AIRreview.APIURL)
	}
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

// DetectProviderFromURL maps an API URL to a registered provider name.
// Uses pattern matching on common provider endpoints.
func DetectProviderFromURL(apiURL string) string {
	u := strings.ToLower(apiURL)
	switch {
	case strings.Contains(u, "anthropic.com"):
		return "anthropic"
	case strings.Contains(u, "googleapis.com"), strings.Contains(u, "generativelanguage"):
		return "gemini"
	default:
		return "openai"
	}
}

func init() {
	RegisterProvider("openrouter", func() Provider { return NewOpenAIProvider() })
	RegisterProvider("openai", func() Provider { return NewOpenAIProvider() })
	RegisterProvider("anthropic", func() Provider { return NewAnthropicProvider() })
	RegisterProvider("gemini", func() Provider { return NewGeminiProvider() })
}
