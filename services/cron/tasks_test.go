package cron

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsScheduleWithSeconds(t *testing.T) {
	tests := []struct {
		schedule  string
		hasSecond bool
	}{
		{"* * * * * *", true},
		{"* * * * *", false},
		{"5 4 * * *", false},
		{"5 4 * * *", false},
		{"5,8 4 * * *", false},
		{"*   *   *  * * *", true},
		{"5,8 4   *  *   *", false},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			assert.Equal(t, test.hasSecond, isScheduleWithSeconds(test.schedule))
		})
	}
}
