// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

const exampleBlame = `
4b92a6c2df28054ad766bc262f308db9f6066596 1 1 1
author Unknown
author-mail <joe2010xtmf@163.com>
author-time 1392833071
author-tz -0500
committer Unknown
committer-mail <joe2010xtmf@163.com>
committer-time 1392833071
committer-tz -0500
summary Add code of delete user
previous be0ba9ea88aff8a658d0495d36accf944b74888d gogs.go
filename gogs.go
	// Copyright 2014 The Gogs Authors. All rights reserved.
ce21ed6c3490cdfad797319cbb1145e2330a8fef 2 2 1
author Joubert RedRat
author-mail <eu+github@redrat.com.br>
author-time 1482322397
author-tz -0200
committer Lunny Xiao
committer-mail <xiaolunwen@gmail.com>
committer-time 1482322397
committer-tz +0800
summary Remove remaining Gogs reference on locales and cmd (#430)
previous 618407c018cdf668ceedde7454c42fb22ba422d8 main.go
filename main.go
	// Copyright 2016 The Gitea Authors. All rights reserved.
4b92a6c2df28054ad766bc262f308db9f6066596 2 3 2
author Unknown
author-mail <joe2010xtmf@163.com>
author-time 1392833071
author-tz -0500
committer Unknown
committer-mail <joe2010xtmf@163.com>
committer-time 1392833071
committer-tz -0500
summary Add code of delete user
previous be0ba9ea88aff8a658d0495d36accf944b74888d gogs.go
filename gogs.go
	// Use of this source code is governed by a MIT-style
4b92a6c2df28054ad766bc262f308db9f6066596 3 4
author Unknown
author-mail <joe2010xtmf@163.com>
author-time 1392833071
author-tz -0500
committer Unknown
committer-mail <joe2010xtmf@163.com>
committer-time 1392833071
committer-tz -0500
summary Add code of delete user
previous be0ba9ea88aff8a658d0495d36accf944b74888d gogs.go
filename gogs.go
	// license that can be found in the LICENSE file.
	
e2aa991e10ffd924a828ec149951f2f20eecead2 6 6 2
author Lunny Xiao
author-mail <xiaolunwen@gmail.com>
author-time 1478872595
author-tz +0800
committer Sandro Santilli
committer-mail <strk@kbt.io>
committer-time 1478872595
committer-tz +0100
summary ask for go get from code.gitea.io/gitea and change gogs to gitea on main file (#146)
previous 5fc370e332171b8658caed771b48585576f11737 main.go
filename main.go
	// Gitea (git with a cup of tea) is a painless self-hosted Git Service.
e2aa991e10ffd924a828ec149951f2f20eecead2 7 7
	package main // import "code.gitea.io/gitea"
`

func TestReadingBlameOutput(t *testing.T) {
	tempFile, err := ioutil.TempFile("", ".txt")
	if err != nil {
		panic(err)
	}

	defer tempFile.Close()

	if _, err = tempFile.WriteString(exampleBlame); err != nil {
		panic(err)
	}

	blameReader, err := createBlameReader("", "cat", tempFile.Name())
	if err != nil {
		panic(err)
	}
	defer blameReader.Close()

	parts := []*BlamePart{
		{
			"4b92a6c2df28054ad766bc262f308db9f6066596",
			[]string{
				"// Copyright 2014 The Gogs Authors. All rights reserved.",
			},
		},
		{
			"ce21ed6c3490cdfad797319cbb1145e2330a8fef",
			[]string{
				"// Copyright 2016 The Gitea Authors. All rights reserved.",
			},
		},
		{
			"4b92a6c2df28054ad766bc262f308db9f6066596",
			[]string{
				"// Use of this source code is governed by a MIT-style",
				"// license that can be found in the LICENSE file.",
				"",
			},
		},
		{
			"e2aa991e10ffd924a828ec149951f2f20eecead2",
			[]string{
				"// Gitea (git with a cup of tea) is a painless self-hosted Git Service.",
				"package main // import \"code.gitea.io/gitea\"",
			},
		},
		nil,
	}

	for _, part := range parts {
		actualPart, err := blameReader.NextPart()
		if err != nil {
			panic(err)
		}
		assert.Equal(t, part, actualPart)
	}
}
