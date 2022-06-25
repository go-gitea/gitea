// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package i18n

import (
	"code.gitea.io/gitea/modules/translation/i18n/common"
	"code.gitea.io/gitea/modules/translation/i18n/dev"
	"code.gitea.io/gitea/modules/translation/i18n/production"
)

var (
	ErrLocaleAlreadyExist = common.ErrLocaleAlreadyExist

	DefaultLocales = NewLocaleStore(false)
)

func NewLocaleStore(isProd bool) LocaleStore {
	if isProd {
		return production.NewLocaleStore()
	}
	return dev.NewLocaleStore()
}

// ResetDefaultLocales resets the current default locales
// NOTE: this is not synchronized
func ResetDefaultLocales(isProd bool) {
	_ = DefaultLocales.Close()
	DefaultLocales = NewLocaleStore(isProd)
}

// Tr use default locales to translate content to target language.
func Tr(lang, trKey string, trArgs ...interface{}) string {
	return DefaultLocales.Tr(lang, trKey, trArgs...)
}
