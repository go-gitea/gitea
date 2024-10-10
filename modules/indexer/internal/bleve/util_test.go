// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBleveGuessFuzzinessByKeyword(t *testing.T) {
	scenarios := []struct {
		Input     string
		Fuzziness int // See util.go for the definition of fuzziness in this particular context
	}{
		{
			Input:     "",
			Fuzziness: 0,
		},
		{
			Input:     "Avocado",
			Fuzziness: 1,
		},
		{
			Input:     "Geschwindigkeit",
			Fuzziness: 2,
		},
		{
			Input:     "non-exist",
			Fuzziness: 0,
		},
		{
			Input:     "갃갃갃",
			Fuzziness: 0,
		},
	}

	for _, scenario := range scenarios {
		t.Run(fmt.Sprintf("ensure fuzziness of '%s' is '%d'", scenario.Input, scenario.Fuzziness), func(t *testing.T) {
			assert.Equal(t, scenario.Fuzziness, GuessFuzzinessByKeyword(scenario.Input))
		})
	}
}
