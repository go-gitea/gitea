// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"testing"

	"code.gitea.io/gitea/modules/container"

	"github.com/stretchr/testify/assert"
)

func TestParseAcceptEncoding(t *testing.T) {
	kases := []struct {
		Header   string
		Expected container.Set[string]
	}{
		{
			Header:   "deflate, gzip;q=1.0, *;q=0.5",
			Expected: container.SetOf("deflate", "gzip"),
		},
		{
			Header:   " gzip, deflate, br",
			Expected: container.SetOf("deflate", "gzip", "br"),
		},
	}

	for _, kase := range kases {
		t.Run(kase.Header, func(t *testing.T) {
			assert.EqualValues(t, kase.Expected, parseAcceptEncoding(kase.Header))
		})
	}
}
