// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hash

const DefaultHashAlgorithmName = "pbkdf2"

var DefaultHashAlgorithm *PasswordHashAlgorithm

var aliasAlgorithmNames = map[string]string{
	"argon2":    "argon2$2$65536$8$50",
	"bcrypt":    "bcrypt$10",
	"scrypt":    "scrypt$65536$16$2$50",
	"pbkdf2":    "pbkdf2_v2", // pbkdf2 should default to pbkdf2_v2
	"pbkdf2_v1": "pbkdf2$10000$50",
	// The latest PBKDF2 password algorithm is used as the default since it doesn't
	// use a lot of  memory and is safer to use on less powerful devices.
	"pbkdf2_v2": "pbkdf2$50000$50",
	// The pbkdf2_hi password algorithm is offered as a stronger alternative to the
	// slightly improved pbkdf2_v2 algorithm
	"pbkdf2_hi": "pbkdf2$320000$50",
}

var RecommendedHashAlgorithms = []string{
	"pbkdf2",
	"argon2",
	"bcrypt",
	"scrypt",
	"pbkdf2_hi",
}

func SetDefaultPasswordHashAlgorithm(algorithmName string) (string, *PasswordHashAlgorithm) {
	if algorithmName == "" {
		algorithmName = DefaultHashAlgorithmName
	}
	alias, has := aliasAlgorithmNames[algorithmName]
	for has {
		algorithmName = alias
		alias, has = aliasAlgorithmNames[algorithmName]
	}
	DefaultHashAlgorithm = Parse(algorithmName)

	return algorithmName, DefaultHashAlgorithm
}
