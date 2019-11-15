// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

var (
	// Migrations settings
	Migrations = struct {
		RetryMaxTimes int
		RetryDelay    int
	}{
		RetryMaxTimes: 2, // after failure, it will try again x times.
		RetryDelay:    3, // delay seconds before next retry
	}
)

func newMigrationsService() {
	sec := Cfg.Section("migrations")
	Migrations.RetryMaxTimes = sec.Key("RETRY_MAX_TIMES").MustInt(Migrations.RetryMaxTimes)
	Migrations.RetryDelay = sec.Key("RETRY_DELAY").MustInt(Migrations.RetryDelay)
}
