// Copyright 2021 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestGenerateMessageID(t *testing.T) {
	var mailService = setting.Mailer{
		From: "test@gitea.com",
	}

	setting.MailService = &mailService
	setting.Domain = "localhost"

	date := time.Date(2000, 01, 02, 03, 04, 05, 06, time.UTC)
	m := NewMessageFrom(nil, "display-name", "from-address", "subject", "body")
	m.Date = date
	gm := m.ToMessage()
	assert.Equal(t, "<autogen-946782245000-41e8fc54a8ad3a3f@localhost>", gm.GetHeader("Message-ID")[0])

	m = NewMessageFrom([]string{"a@b.com"}, "display-name", "from-address", "subject", "body")
	m.Date = date
	gm = m.ToMessage()
	assert.Equal(t, "<autogen-946782245000-cc88ce3cfe9bd04f@localhost>", gm.GetHeader("Message-ID")[0])

	m = NewMessageFrom([]string{"a@b.com"}, "display-name", "from-address", "subject", "body")
	m.SetHeader("Message-ID", "<msg-d@domain.com>")
	gm = m.ToMessage()
	assert.Equal(t, "<msg-d@domain.com>", gm.GetHeader("Message-ID")[0])
}

func TestCRLFConverter(t *testing.T) {
	type testcaseType struct {
		input    []string
		expected string
	}
	testcases := []testcaseType{
		{
			input:    []string{"This h\ras a \r", "\nnewline\r\n"},
			expected: "This h\ras a \nnewline\n",
		},
		{
			input:    []string{"This\r\n has a \r\n\r", "\n\r\nnewline\r\n"},
			expected: "This\n has a \n\n\nnewline\n",
		},
		{
			input:    []string{"This has a \r", "\nnewline\r"},
			expected: "This has a \nnewline\r",
		},
		{
			input:    []string{"This has a \r", "newline\r"},
			expected: "This has a \rnewline\r",
		},
	}
	for _, testcase := range testcases {
		out := &strings.Builder{}
		converter := &crlfConverter{w: out}
		realsum, sum := 0, 0
		for _, in := range testcase.input {
			n, err := converter.Write([]byte(in))
			assert.NoError(t, err)
			assert.Equal(t, len(in), n)
			sum += n
			realsum += len(in)
		}
		err := converter.Close()
		assert.NoError(t, err)
		assert.Equal(t, realsum, sum)
		assert.Equal(t, testcase.expected, out.String())
	}
}
