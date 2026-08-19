// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package validation

import (
	"context"
	"io"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"gitea.dev/modules/auth"
	"gitea.dev/modules/git"
	"gitea.dev/modules/glob"
	"gitea.dev/modules/json"
	"gitea.dev/modules/util"

	"gitea.com/go-chi/binding" //nolint:depguard // this package wraps it
)

const (
	ErrCustomMessage       = "CustomMessage"
	ErrGitRefName          = "GitRefNameError"
	ErrGlobPattern         = "GlobPattern"
	ErrRegexPattern        = "RegexPattern"
	ErrUsername            = "UsernameError"
	ErrInvalidGroupTeamMap = "InvalidGroupTeamMap"
	ErrInvalidBadgeSlug    = "InvalidBadgeSlug"
)

type jsonProvider struct{}

func (j jsonProvider) Marshal(v any) ([]byte, error) { return json.Marshal(v) }

func (j jsonProvider) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

func (j jsonProvider) NewDecoder(reader io.Reader) binding.JSONDecoder {
	return json.NewDecoder(reader)
}

func (j jsonProvider) NewEncoder(writer io.Writer) binding.JSONEncoder {
	return json.NewEncoder(writer)
}

func newFieldError(field reflect.StructField, cls, msg string) *BindingError {
	return &BindingError{[]string{field.Name}, cls, msg} //nolint:govet // make sure no missing fields
}

// AddBindingRules adds additional binding rules
func AddBindingRules(b *binding.Binder) {
	b.AddRuleNonZero("GitRefName", func(ctx context.Context, f *binding.ValidationField) *binding.Error {
		if !git.IsValidRefPattern(f.ValueMustString()) {
			return newFieldError(f.StructField, ErrGitRefName, "GitRefName")
		}
		return nil
	})
	b.AddRuleNonZero("ValidUrl", func(ctx context.Context, f *binding.ValidationField) *binding.Error {
		if !IsValidURL(f.ValueMustString()) {
			return newFieldError(f.StructField, binding.ERR_URL, "Url")
		}
		return nil
	})
	b.AddRuleNonZero("ValidSiteUrl", func(ctx context.Context, f *binding.ValidationField) *binding.Error {
		if !IsValidSiteURL(f.ValueMustString()) {
			return newFieldError(f.StructField, binding.ERR_URL, "Url")
		}
		return nil
	})
	b.AddRuleNonZero("BadgeSlug", func(ctx context.Context, f *binding.ValidationField) *binding.Error {
		if !IsValidBadgeSlug(f.ValueMustString()) {
			return newFieldError(f.StructField, ErrInvalidBadgeSlug, "invalid badge slug")
		}
		return nil
	})

	ruleGlobPattern := func(_ context.Context, f *binding.ValidationField) *binding.Error {
		if _, err := glob.Compile(f.ValueMustString()); err != nil {
			return newFieldError(f.StructField, ErrGlobPattern, err.Error())
		}
		return nil
	}
	b.AddRuleNonZero("GlobPattern", ruleGlobPattern)
	ruleRegexPattern := func(_ context.Context, f *binding.ValidationField, val string) *binding.Error {
		if _, err := regexp.Compile(val); err != nil {
			return newFieldError(f.StructField, ErrRegexPattern, err.Error())
		}
		return nil
	}
	b.AddRuleNonZero("RegexPattern", func(ctx context.Context, f *binding.ValidationField) *binding.Error {
		return ruleRegexPattern(ctx, f, f.ValueMustString())
	})
	b.AddRuleNonZero("GlobOrRegexPattern", func(ctx context.Context, f *binding.ValidationField) *binding.Error {
		str := f.ValueMustString()
		if len(str) >= 2 && strings.HasPrefix(str, "/") && strings.HasSuffix(str, "/") {
			return ruleRegexPattern(ctx, f, str[1:len(str)-1])
		}
		return ruleGlobPattern(ctx, f)
	})

	b.AddRuleNonZero("Username", func(ctx context.Context, f *binding.ValidationField) *binding.Error {
		if !IsValidUsername(f.ValueMustString()) {
			return newFieldError(f.StructField, ErrUsername, "invalid username")
		}
		return nil
	})

	b.AddRuleNonZero("ValidGroupTeamMap", func(ctx context.Context, f *binding.ValidationField) *binding.Error {
		_, err := auth.UnmarshalGroupTeamMapping(f.ValueMustString())
		if err != nil {
			return newFieldError(f.StructField, ErrInvalidGroupTeamMap, err.Error())
		}
		return nil
	})
}

func portOnly(hostport string) string {
	_, after, ok := strings.Cut(hostport, ":")
	if !ok {
		return ""
	}
	if _, after2, ok2 := strings.Cut(hostport, "]:"); ok2 {
		return after2
	}
	if strings.Contains(hostport, "]") {
		return ""
	}
	return after
}

func validPort(p string) bool {
	for _, r := range []byte(p) {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

var Binder = sync.OnceValue(func() *binding.Binder {
	b := binding.NewBinder().WithJSONProvider(jsonProvider{}).WithDefaultRules().WithNameMapper(util.ToSnakeCase)
	AddBindingRules(b)
	return b
})

type (
	BindingErrors = binding.Errors
	BindingError  = binding.Error
)
