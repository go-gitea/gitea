// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"bytes"
	"html/template"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderInternal(t *testing.T) {
	cases := []struct {
		input, protected, recovered string
	}{
		{
			input:     `<div class="test">class="content"</div>`,
			protected: `<div data-attr-class="sec:test">class="content"</div>`,
			recovered: `<div class="test">class="content"</div>`,
		},
		{
			input:     "<div\nclass=\"test\" data-xxx></div>",
			protected: `<div data-attr-class="sec:test" data-xxx></div>`,
			recovered: `<div class="test" data-xxx></div>`,
		},
	}
	for _, c := range cases {
		var r RenderInternal
		out := &bytes.Buffer{}
		in := r.init("sec", out)
		protected := r.ProtectSafeAttrs(template.HTML(c.input))
		assert.EqualValues(t, c.protected, protected)
		_, _ = io.WriteString(in, string(protected))
		_ = in.Close()
		assert.EqualValues(t, c.recovered, out.String())
	}

	var r1, r2 RenderInternal
	protected := r1.ProtectSafeAttrs(`<div class="test"></div>`)
	assert.EqualValues(t, `<div class="test"></div>`, protected, "non-initialized RenderInternal should not protect any attributes")
	_ = r1.init("sec", nil)
	protected = r1.ProtectSafeAttrs(`<div class="test"></div>`)
	assert.EqualValues(t, `<div data-attr-class="sec:test"></div>`, protected)
	assert.EqualValues(t, "data-attr-class", r1.SafeAttr("class"))
	assert.EqualValues(t, "sec:val", r1.SafeValue("val"))
	recovered, ok := r1.RecoverProtectedValue("sec:val")
	assert.True(t, ok)
	assert.EqualValues(t, "val", recovered)
	recovered, ok = r1.RecoverProtectedValue("other:val")
	assert.False(t, ok)
	assert.Empty(t, recovered)

	out2 := &bytes.Buffer{}
	in2 := r2.init("sec-other", out2)
	_, _ = io.WriteString(in2, string(protected))
	_ = in2.Close()
	assert.EqualValues(t, `<div data-attr-class="sec:test"></div>`, out2.String(), "different secureID should not recover the value")
}
