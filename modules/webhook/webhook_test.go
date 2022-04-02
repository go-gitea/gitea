// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"bytes"
	"testing"

	"code.gitea.io/gitea/testdata"

	"github.com/stretchr/testify/assert"
)

func TestWebhook(t *testing.T) {
	tt := []struct {
		Name string
		File string
		Err  bool
	}{
		{
			Name: "Executable",
			File: "executable.yml",
		},
		{
			Name: "HTTP",
			File: "http.yml",
		},
		{
			Name: "Bad",
			File: "bad.yml",
			Err:  true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			contents, err := testdata.Webhook.ReadFile("webhook/" + tc.File)
			assert.NoError(t, err, "expected to read file")

			_, err = Parse(bytes.NewReader(contents))
			if tc.Err {
				assert.Error(t, err, "expected to get an error")
			} else {
				assert.NoError(t, err, "expected to not get an error")
			}
		})
	}
}
