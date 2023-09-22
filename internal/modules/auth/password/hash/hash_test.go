// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hash

import (
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testSaltHasher string

func (t testSaltHasher) HashWithSaltBytes(password string, salt []byte) string {
	return password + "$" + string(salt) + "$" + string(t)
}

func Test_registerHasher(t *testing.T) {
	MustRegister("Test_registerHasher", func(config string) testSaltHasher {
		return testSaltHasher(config)
	})

	assert.Panics(t, func() {
		MustRegister("Test_registerHasher", func(config string) testSaltHasher {
			return testSaltHasher(config)
		})
	})

	assert.Error(t, Register("Test_registerHasher", func(config string) testSaltHasher {
		return testSaltHasher(config)
	}))

	assert.Equal(t, "password$salt$",
		Parse("Test_registerHasher").PasswordSaltHasher.HashWithSaltBytes("password", []byte("salt")))

	assert.Equal(t, "password$salt$config",
		Parse("Test_registerHasher$config").PasswordSaltHasher.HashWithSaltBytes("password", []byte("salt")))

	delete(availableHasherFactories, "Test_registerHasher")
}

func TestParse(t *testing.T) {
	hashAlgorithmsToTest := []string{}
	for plainHashAlgorithmNames := range availableHasherFactories {
		hashAlgorithmsToTest = append(hashAlgorithmsToTest, plainHashAlgorithmNames)
	}
	for _, aliased := range aliasAlgorithmNames {
		if strings.Contains(aliased, "$") {
			hashAlgorithmsToTest = append(hashAlgorithmsToTest, aliased)
		}
	}
	for _, algorithmName := range hashAlgorithmsToTest {
		t.Run(algorithmName, func(t *testing.T) {
			algo := Parse(algorithmName)
			assert.NotNil(t, algo, "Algorithm %s resulted in an empty algorithm", algorithmName)
		})
	}
}

func TestHashing(t *testing.T) {
	hashAlgorithmsToTest := []string{}
	for plainHashAlgorithmNames := range availableHasherFactories {
		hashAlgorithmsToTest = append(hashAlgorithmsToTest, plainHashAlgorithmNames)
	}
	for _, aliased := range aliasAlgorithmNames {
		if strings.Contains(aliased, "$") {
			hashAlgorithmsToTest = append(hashAlgorithmsToTest, aliased)
		}
	}

	runTests := func(password, salt string, shouldPass bool) {
		for _, algorithmName := range hashAlgorithmsToTest {
			t.Run(algorithmName, func(t *testing.T) {
				output, err := Parse(algorithmName).Hash(password, salt)
				if shouldPass {
					assert.NoError(t, err)
					assert.NotEmpty(t, output, "output for %s was empty", algorithmName)
				} else {
					assert.Error(t, err)
				}

				assert.Equal(t, Parse(algorithmName).VerifyPassword(password, output, salt), shouldPass)
			})
		}
	}

	// Test with new salt format.
	runTests(strings.Repeat("a", 16), hex.EncodeToString([]byte{0x01, 0x02, 0x03}), true)

	// Test with legacy salt format.
	runTests(strings.Repeat("a", 16), strings.Repeat("b", 10), true)

	// Test with invalid salt.
	runTests(strings.Repeat("a", 16), "a", false)
}

// vectors were generated using the current codebase.
var vectors = []struct {
	algorithms []string
	password   string
	salt       string
	output     string
	shouldfail bool
}{
	{
		algorithms: []string{"bcrypt", "bcrypt$10"},
		password:   "abcdef",
		salt:       strings.Repeat("a", 10),
		output:     "$2a$10$fjtm8BsQ2crym01/piJroenO3oSVUBhSLKaGdTYJ4tG0ePVCrU0G2",
		shouldfail: false,
	},
	{
		algorithms: []string{"scrypt", "scrypt$65536$16$2$50"},
		password:   "abcdef",
		salt:       strings.Repeat("a", 10),
		output:     "3b571d0c07c62d42b7bad3dbf18fb0cd67d4d8cd4ad4c6928e1090e5b2a4a84437c6fd2627d897c0e7e65025ca62b67a0002",
		shouldfail: false,
	},
	{
		algorithms: []string{"argon2", "argon2$2$65536$8$50"},
		password:   "abcdef",
		salt:       strings.Repeat("a", 10),
		output:     "551f089f570f989975b6f7c6a8ff3cf89bc486dd7bbe87ed4d80ad4362f8ee599ec8dda78dac196301b98456402bcda775dc",
		shouldfail: false,
	},
	{
		algorithms: []string{"pbkdf2", "pbkdf2$10000$50"},
		password:   "abcdef",
		salt:       strings.Repeat("a", 10),
		output:     "ab48d5471b7e6ed42d10001db88c852ff7303c788e49da5c3c7b63d5adf96360303724b74b679223a3dea8a242d10abb1913",
		shouldfail: false,
	},
	{
		algorithms: []string{"bcrypt", "bcrypt$10"},
		password:   "abcdef",
		salt:       hex.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04}),
		output:     "$2a$10$qhgm32w9ZpqLygugWJsLjey8xRGcaq9iXAfmCeNBXxddgyoaOC3Gq",
		shouldfail: false,
	},
	{
		algorithms: []string{"scrypt", "scrypt$65536$16$2$50"},
		password:   "abcdef",
		salt:       hex.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04}),
		output:     "25fe5f66b43fa4eb7b6717905317cd2223cf841092dc8e0a1e8c75720ad4846cb5d9387303e14bc3c69faa3b1c51ef4b7de1",
		shouldfail: false,
	},
	{
		algorithms: []string{"argon2", "argon2$2$65536$8$50"},
		password:   "abcdef",
		salt:       hex.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04}),
		output:     "9c287db63a91d18bb1414b703216da4fc431387c1ae7c8acdb280222f11f0929831055dbfd5126a3b48566692e83ec750d2a",
		shouldfail: false,
	},
	{
		algorithms: []string{"pbkdf2", "pbkdf2$10000$50"},
		password:   "abcdef",
		salt:       hex.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04}),
		output:     "45d6cdc843d65cf0eda7b90ab41435762a282f7df013477a1c5b212ba81dbdca2edf1ecc4b5cb05956bb9e0c37ab29315d78",
		shouldfail: false,
	},
	{
		algorithms: []string{"pbkdf2$320000$50"},
		password:   "abcdef",
		salt:       hex.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04}),
		output:     "84e233114499e8721da80e85568e5b7b5900b3e49a30845fcda9d1e1756da4547d70f8740ac2b4a5d82f88cebcd27f21bfe2",
		shouldfail: false,
	},
	{
		algorithms: []string{"pbkdf2", "pbkdf2$10000$50"},
		password:   "abcdef",
		salt:       "",
		output:     "",
		shouldfail: true,
	},
}

// Ensure that the current code will correctly verify against the test vectors.
func TestVectors(t *testing.T) {
	for i, vector := range vectors {
		for _, algorithm := range vector.algorithms {
			t.Run(strconv.Itoa(i)+": "+algorithm, func(t *testing.T) {
				pa := Parse(algorithm)
				assert.Equal(t, !vector.shouldfail, pa.VerifyPassword(vector.password, vector.output, vector.salt))
			})
		}
	}
}
