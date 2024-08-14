// Copyright 2017 The Gitea Authors. All rights reserved.
// Copyright 2017 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"regexp"
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

// Sanitizer is a protection wrapper of *bluemonday.Policy which does not allow
// any modification to the underlying policies once it's been created.
type Sanitizer struct {
	defaultPolicy     *bluemonday.Policy
	descriptionPolicy *bluemonday.Policy
	rendererPolicies  map[string]*bluemonday.Policy
	allowAllRegex     *regexp.Regexp
}

var (
	defaultSanitizer     *Sanitizer
	defaultSanitizerOnce sync.Once
)

func GetDefaultSanitizer() *Sanitizer {
	defaultSanitizerOnce.Do(func() {
		defaultSanitizer = &Sanitizer{
			rendererPolicies: map[string]*bluemonday.Policy{},
			allowAllRegex:    regexp.MustCompile(".+"),
		}
		for name, renderer := range renderers {
			sanitizerRules := renderer.SanitizerRules()
			if len(sanitizerRules) > 0 {
				policy := defaultSanitizer.createDefaultPolicy()
				defaultSanitizer.addSanitizerRules(policy, sanitizerRules)
				defaultSanitizer.rendererPolicies[name] = policy
			}
		}
		defaultSanitizer.defaultPolicy = defaultSanitizer.createDefaultPolicy()
		defaultSanitizer.descriptionPolicy = defaultSanitizer.createRepoDescriptionPolicy()
	})
	return defaultSanitizer
}

func ResetDefaultSanitizerForTesting() {
	defaultSanitizer = nil
	defaultSanitizerOnce = sync.Once{}
}
