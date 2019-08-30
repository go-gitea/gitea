// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"fmt"
)

var Migration = struct {
	WhitelistedDomains []string
	BlacklistedDomains []string
}{
	WhitelistedDomains: []string{},
	BlacklistedDomains: []string{},
}

// InitMigrationConfig represents load migration configurations
func InitMigrationConfig() error {
	if err := Cfg.Section("migration").MapTo(&Migration); err != nil {
		return fmt.Errorf("Failed to map Migration settings: %v", err)
	}
	return nil
}
