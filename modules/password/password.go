// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package password

import (
	"crypto/rand"
	"math/big"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/setting"
)

var (
	matchComplexityOnce sync.Once
	validChars          string
	requiredChars       []string

	charComplexities = map[string]string{
		"lower": `abcdefghijklmnopqrstuvwxyz`,
		"upper": `ABCDEFGHIJKLMNOPQRSTUVWXYZ`,
		"digit": `0123456789`,
		"spec":  ` !"#$%&'()*+,-./:;<=>?@[\]^_{|}~` + "`",
	}
)

// NewComplexity for preparation
func NewComplexity() {
	matchComplexityOnce.Do(func() {
		setupComplexity(setting.PasswordComplexity)
	})
}

func setupComplexity(values []string) {
	if len(values) != 1 || values[0] != "off" {
		for _, val := range values {
			if chars, ok := charComplexities[val]; ok {
				validChars += chars
				requiredChars = append(requiredChars, chars)
			}
		}
		if len(requiredChars) == 0 {
			// No valid character classes found; use all classes as default
			for _, chars := range charComplexities {
				validChars += chars
				requiredChars = append(requiredChars, chars)
			}
		}
	}
	if validChars == "" {
		// No complexities to check; provide a sensible default for password generation
		validChars = charComplexities["lower"] + charComplexities["upper"] + charComplexities["digit"]
	}
}

// IsComplexEnough return True if password meets complexity settings
func IsComplexEnough(pwd string) bool {
	NewComplexity()
	if len(validChars) > 0 {
		for _, req := range requiredChars {
			if !strings.ContainsAny(req, pwd) {
				return false
			}
		}
	}
	return true
}

// Generate  a random password
func Generate(n int) (string, error) {
	NewComplexity()
	buffer := make([]byte, n)
	max := big.NewInt(int64(len(validChars)))
	for {
		for j := 0; j < n; j++ {
			rnd, err := rand.Int(rand.Reader, max)
			if err != nil {
				return "", err
			}
			buffer[j] = validChars[rnd.Int64()]
		}
		if IsComplexEnough(string(buffer)) && string(buffer[0]) != " " && string(buffer[n-1]) != " " {
			return string(buffer), nil
		}
	}
}
