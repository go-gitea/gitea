// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const exampleBlame = `4b92a6c2df gogs.go  1) // Copyright 2014 The Gogs Authors. All rights reserved.
ce21ed6c34 main.go  2) // Copyright 2016 The Gitea Authors. All rights reserved.
4b92a6c2df gogs.go  3) // Use of this source code is governed by a MIT-style
4b92a6c2df gogs.go  4) // license that can be found in the LICENSE file.
be0ba9ea88 gogs.go  5) 
e2aa991e10 main.go  6) // Gitea (git with a cup of tea) is a painless self-hosted Git Service.
e2aa991e10 main.go  7) package main // import "code.gitea.io/gitea"
`

func TestReadingBlameOutput(t *testing.T) {
	// panic(fmt.Errorf("TEST"))

	blame, err := parseBlameOutput(strings.NewReader(exampleBlame))

	if err != nil {
		panic(err)
	}

	assert.Equal(t, &BlameFile{
		[]BlamePart{
			{
				"4b92a6c2df",
				[]string{
					"// Copyright 2014 The Gogs Authors. All rights reserved.",
				},
			},
			{
				"ce21ed6c34",
				[]string{
					"// Copyright 2016 The Gitea Authors. All rights reserved.",
				},
			},
			{
				"4b92a6c2df",
				[]string{
					"// Use of this source code is governed by a MIT-style",
					"// license that can be found in the LICENSE file.",
				},
			},
			{
				"be0ba9ea88",
				[]string{
					"",
				},
			},
			{
				"e2aa991e10",
				[]string{
					"// Gitea (git with a cup of tea) is a painless self-hosted Git Service.",
					"package main // import \"code.gitea.io/gitea\"",
				},
			},
		},
	}, blame)

}
