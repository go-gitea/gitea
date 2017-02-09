package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_StatusTable(t *testing.T) {
	table := NewStatusTable()

	assert.False(t, table.IsRunning("xyz"))

	table.Start("xyz")
	assert.True(t, table.IsRunning("xyz"))

	table.Stop("xyz")
	assert.False(t, table.IsRunning("xyz"))
}
