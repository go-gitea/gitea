// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hash

// DefaultHashAlgorithmName represents the default value of PASSWORD_HASH_ALGO
// configured in app.ini.
//
// It is NOT the same and does NOT map to the defaultEmptyHashAlgorithmSpecification.
//
// It will be dealiased as per aliasAlgorithmNames whereas
// defaultEmptyHashAlgorithmSpecification does not undergo dealiasing.
const DefaultHashAlgorithmName = "pbkdf2"

var DefaultHashAlgorithm *PasswordHashAlgorithm

// aliasAlgorithNames provides a mapping between the value of PASSWORD_HASH_ALGO
// configured in the app.ini and the parameters used within the hashers internally.
//
// If it is necessary to change the default parameters for any hasher in future you
// should change these values and not those in argon2.go etc.
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

// hashAlgorithmToSpec converts an algorithm name or a specification to a full algorithm specification
func hashAlgorithmToSpec(algorithmName string) string {
	if algorithmName == "" {
		algorithmName = DefaultHashAlgorithmName
	}
	alias, has := aliasAlgorithmNames[algorithmName]
	for has {
		algorithmName = alias
		alias, has = aliasAlgorithmNames[algorithmName]
	}
	return algorithmName
}

// SetDefaultPasswordHashAlgorithm will take a provided algorithmName and de-alias it to
// a complete algorithm specification.
func SetDefaultPasswordHashAlgorithm(algorithmName string) (string, *PasswordHashAlgorithm) {
	algoSpec := hashAlgorithmToSpec(algorithmName)
	// now we get a full specification, e.g. pbkdf2$50000$50 rather than pbdkf2
	DefaultHashAlgorithm = Parse(algoSpec)
	return algoSpec, DefaultHashAlgorithm
}

// ConfigHashAlgorithm will try to find a "recommended algorithm name" defined by RecommendedHashAlgorithms for config
// This function is not fast and is only used for the installation page
func ConfigHashAlgorithm(algorithm string) string {
	algorithm = hashAlgorithmToSpec(algorithm)
	for _, recommAlgo := range RecommendedHashAlgorithms {
		if algorithm == hashAlgorithmToSpec(recommAlgo) {
			return recommAlgo
		}
	}
	return algorithm
}
