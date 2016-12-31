package process

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManager_Add(t *testing.T) {
	pm := Manager{Processes: make(map[int64]*Process)}

	pid := pm.Add("foo", exec.Command("foo"))
	assert.Equal(t, int64(1), pid, "expected to get pid 1 got %d", pid)

	pid = pm.Add("bar", exec.Command("bar"))
	assert.Equal(t, int64(2), pid, "expected to get pid 2 got %d", pid)
}

func TestManager_Remove(t *testing.T) {
	pm := Manager{Processes: make(map[int64]*Process)}

	pid1 := pm.Add("foo", exec.Command("foo"))
	assert.Equal(t, int64(1), pid1, "expected to get pid 1 got %d", pid1)

	pid2 := pm.Add("bar", exec.Command("bar"))
	assert.Equal(t, int64(2), pid2, "expected to get pid 2 got %d", pid2)

	pm.Remove(pid2)

	_, exists := pm.Processes[pid2]
	assert.False(t, exists, "PID %d is in the list but shouldn't", pid2)
}
