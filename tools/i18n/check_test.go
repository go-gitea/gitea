package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_FindTemplateKeys(t *testing.T) {
	path := "../../templates/repo/issue/view_content/comments.tmpl"
	expectedKeys := []string{

	}

	keys, err := FindTemplateKeys(path)
	assert.NoError(t, err)
}
