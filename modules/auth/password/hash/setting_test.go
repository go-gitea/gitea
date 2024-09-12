// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hash

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckSettingPasswordHashAlgorithm(t *testing.T) {
	t.Run("pbkdf2 is pbkdf2_v2", func(t *testing.T) {
		pbkdf2v2Config, pbkdf2v2Algo := SetDefaultPasswordHashAlgorithm("pbkdf2_v2")
		pbkdf2Config, pbkdf2Algo := SetDefaultPasswordHashAlgorithm("pbkdf2")

		assert.Equal(t, pbkdf2v2Config, pbkdf2Config)
		assert.Equal(t, pbkdf2v2Algo.Specification, pbkdf2Algo.Specification)
	})

	for a, b := range aliasAlgorithmNames {
		t.Run(a+"="+b, func(t *testing.T) {
			aConfig, aAlgo := SetDefaultPasswordHashAlgorithm(a)
			bConfig, bAlgo := SetDefaultPasswordHashAlgorithm(b)

			assert.Equal(t, bConfig, aConfig)
			assert.Equal(t, aAlgo.Specification, bAlgo.Specification)
		})
	}

	t.Run("pbkdf2_v2 is the default when default password hash algorithm is empty", func(t *testing.T) {
		emptyConfig, emptyAlgo := SetDefaultPasswordHashAlgorithm("")
		pbkdf2v2Config, pbkdf2v2Algo := SetDefaultPasswordHashAlgorithm("pbkdf2_v2")

		assert.Equal(t, pbkdf2v2Config, emptyConfig)
		assert.Equal(t, pbkdf2v2Algo.Specification, emptyAlgo.Specification)
	})
}
