// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "fmt"

type DeprecatedWarning struct {
	OldSection string
	OldKey     string
	NewSection string
	NewKey     string
	Version    string
}

func (dw DeprecatedWarning) String() string {
	return fmt.Sprintf("Deprecated fallback `[%s]` `%s` present. Use `[%s]` `%s` instead. This fallback will be/has been removed in %s",
		dw.OldSection, dw.OldKey, dw.NewSection, dw.NewKey, dw.Version)
}

// DeprecatedWarnings all warnings which need administrator to change
var DeprecatedWarnings []DeprecatedWarning
