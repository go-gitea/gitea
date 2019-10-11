// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package password

import (
	"crypto/rand"
	"math/big"
	"regexp"
	"sync"

	"code.gitea.io/gitea/modules/setting"
)

var matchComplexities = map[string]regexp.Regexp{}
var matchComplexityOnce sync.Once
var validChars string
var validComplexities = map[string]string{
	"lower": "abcdefghijklmnopqrstuvwxyz",
	"upper": "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
	"digit": "0123456789",
	"spec":  `][ !"#$%&'()*+,./:;<=>?@\^_{|}~` + "`-",
}

// NewComplexity for preparation
func NewComplexity() {
	matchComplexityOnce.Do(func() {
		if len(setting.PasswordComplexity) > 0 {
			for key, val := range setting.PasswordComplexity {
				matchComplexity := regexp.MustCompile(val)
				matchComplexities[key] = *matchComplexity
				validChars += validComplexities[key]
			}
		} else {
			for _, val := range validComplexities {
				validChars += val
			}
		}
	})
}

// IsComplexEnough return True if password is Complexity
func IsComplexEnough(pwd string) bool {
	if len(setting.PasswordComplexity) > 0 {
		NewComplexity()
		for _, val := range matchComplexities {
			if !val.MatchString(pwd) {
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
