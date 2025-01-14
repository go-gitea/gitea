package arch

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompareVersions(t *testing.T) {
	// https://man.archlinux.org/man/vercmp.8.en
	checks := [][]string{
		{"1.0a", "1.0b", "1.0beta", "1.0p", "1.0pre", "1.0rc", "1.0", "1.0.a", "1.0.1"},
		{"1", "1.0", "1.1", "1.1.1", "1.2", "2.0", "3.0.0"},
	}
	for _, check := range checks {
		for i := 0; i < len(check)-1; i++ {
			require.Equal(t, -1, compareVersions(check[i], check[i+1]), fmt.Sprintf("%s > %s", check[i], check[i+1]))
			require.Equal(t, 1, compareVersions(check[i+1], check[i]), fmt.Sprintf("%s > %s", check[i], check[i+1]))
		}
	}
	require.Equal(t, 1, compareVersions("1.0-2", "1.0"))
	require.Equal(t, 0, compareVersions("0:1.0-1", "1.0"))
	require.Equal(t, 1, compareVersions("1:1.0-1", "2.0"))
}
