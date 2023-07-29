// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

// Messenger is a formatting function similar to i18n.Tr
type Messenger func(key string, args ...any)

// NilMessenger represents an empty formatting function
func NilMessenger(string, ...any) {}
