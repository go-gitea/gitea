// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package password

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"html/template"
	"math/big"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/translation"
)

var (
	ErrComplexity = errors.New("password not complex enough")
	ErrMinLength  = errors.New("password not long enough")
)

// complexity contains information about a particular kind of password complexity
type complexity struct {
	ValidChars string
	TrNameOne  string
}

var (
	matchComplexityOnce sync.Once
	validChars          string
	requiredList        []complexity

	charComplexities = map[string]complexity{
		"lower": {
			`abcdefghijklmnopqrstuvwxyz`,
			"form.password_lowercase_one",
		},
		"upper": {
			`ABCDEFGHIJKLMNOPQRSTUVWXYZ`,
			"form.password_uppercase_one",
		},
		"digit": {
			`0123456789`,
			"form.password_digit_one",
		},
		"spec": {
			` !"#$%&'()*+,-./:;<=>?@[\]^_{|}~` + "`",
			"form.password_special_one",
		},
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
			if complexity, ok := charComplexities[val]; ok {
				validChars += complexity.ValidChars
				requiredList = append(requiredList, complexity)
			}
		}
		if len(requiredList) == 0 {
			// No valid character classes found; use all classes as default
			for _, complexity := range charComplexities {
				validChars += complexity.ValidChars
				requiredList = append(requiredList, complexity)
			}
		}
	}
	if validChars == "" {
		// No complexities to check; provide a sensible default for password generation
		validChars = charComplexities["lower"].ValidChars + charComplexities["upper"].ValidChars + charComplexities["digit"].ValidChars
	}
}

// IsComplexEnough return True if password meets complexity settings
func IsComplexEnough(pwd string) bool {
	NewComplexity()
	if len(validChars) > 0 {
		for _, req := range requiredList {
			if !strings.ContainsAny(req.ValidChars, pwd) {
				return false
			}
		}
	}
	return true
}

// Generate a random password
func Generate(n int) (string, error) {
	NewComplexity()
	buffer := make([]byte, n)
	maxInt := big.NewInt(int64(len(validChars)))
	for {
		for j := 0; j < n; j++ {
			rnd, err := rand.Int(rand.Reader, maxInt)
			if err != nil {
				return "", err
			}
			buffer[j] = validChars[rnd.Int64()]
		}

		if err := IsPwned(context.Background(), string(buffer)); err != nil {
			if errors.Is(err, ErrIsPwned) {
				continue
			}
			return "", err
		}
		if IsComplexEnough(string(buffer)) && string(buffer[0]) != " " && string(buffer[n-1]) != " " {
			return string(buffer), nil
		}
	}
}

// BuildComplexityError builds the error message when password complexity checks fail
func BuildComplexityError(locale translation.Locale) template.HTML {
	var buffer bytes.Buffer
	buffer.WriteString(locale.TrString("form.password_complexity"))
	buffer.WriteString("<ul>")
	for _, c := range requiredList {
		buffer.WriteString("<li>")
		buffer.WriteString(locale.TrString(c.TrNameOne))
		buffer.WriteString("</li>")
	}
	buffer.WriteString("</ul>")
	return template.HTML(buffer.String())
}
