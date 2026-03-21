// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codeformat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatImportsSimple(t *testing.T) {
	formatted, err := formatGoImports([]byte(`
package codeformat

import (
	"github.com/stretchr/testify/assert"
	"testing"
)
`))

	expected := `
package codeformat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)
`

	assert.NoError(t, err)
	assert.Equal(t, expected, string(formatted))
}

func TestFormatImportsGroup(t *testing.T) {
	// gofmt/goimports won't group the packages, for example, they produce such code:
	//     "bytes"
	//     "image"
	//        (a blank line)
	//     "fmt"
	//     "image/color/palette"
	// our formatter does better, and these packages are grouped into one.

	formatted, err := formatGoImports([]byte(`
package test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"

	_ "image/gif"  // for processing gif images
	_ "image/jpeg" // for processing jpeg images
	_ "image/png"  // for processing png images

	"code.gitea.io/other/package"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

  "xorm.io/the/package"

	"github.com/issue9/identicon"
	"github.com/nfnt/resize"
	"github.com/oliamb/cutter"
)
`))

	expected := `
package test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"

	_ "image/gif"  // for processing gif images
	_ "image/jpeg" // for processing jpeg images
	_ "image/png"  // for processing png images

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"code.gitea.io/other/package"
	"github.com/issue9/identicon"
	"github.com/nfnt/resize"
	"github.com/oliamb/cutter"
	"xorm.io/the/package"
)
`

	assert.NoError(t, err)
	assert.Equal(t, expected, string(formatted))
}

func TestFormatImportsInvalidComment(t *testing.T) {
	// why we shouldn't write comments between imports: it breaks the grouping of imports
	// for example:
	//    "pkg1"
	//    "pkg2"
	//    // a comment
	//    "pkgA"
	//    "pkgB"
	// the comment splits the packages into two groups, pkg1/2 are sorted separately, pkgA/B are sorted separately
	// we don't want such code, so the code should be:
	//    "pkg1"
	//    "pkg2"
	//    "pkgA" // a comment
	//    "pkgB"

	_, err := formatGoImports([]byte(`
package test

import (
  "image/jpeg"
	// for processing gif images
	"image/gif"
)
`))
	assert.ErrorIs(t, err, errInvalidCommentBetweenImports)
}
