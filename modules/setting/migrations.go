// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

var (
	// Migrations settings
	Migrations = struct {
		MaxAttempts  int
		RetryBackoff int
	}{
		MaxAttempts:  3,
		RetryBackoff: 3,
	}
)

func newMigrationsService() {
	sec := Cfg.Section("migrations")
	Migrations.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(Migrations.MaxAttempts)
	Migrations.RetryBackoff = sec.Key("RETRY_BACKOFF").MustInt(Migrations.RetryBackoff)
}
