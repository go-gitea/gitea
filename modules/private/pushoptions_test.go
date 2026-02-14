// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitPushOptions(t *testing.T) {
	o := GitPushOptions{}

	v := o.Bool("no-such")
	assert.False(t, v.Has())
	assert.False(t, v.Value())

	o.AddFromKeyValue("opt1=a=b")
	o.AddFromKeyValue("opt2=false")
	o.AddFromKeyValue("opt3=true")
	o.AddFromKeyValue("opt4")

	assert.Equal(t, "a=b", o["opt1"])
	assert.False(t, o.Bool("opt1").Value())
	assert.True(t, o.Bool("opt2").Has())
	assert.False(t, o.Bool("opt2").Value())
	assert.True(t, o.Bool("opt3").Value())
	assert.True(t, o.Bool("opt4").Value())
}
