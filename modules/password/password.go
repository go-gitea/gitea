package password

import (
	"math/rand"
	"regexp"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/setting"
)

var matchComplexities = map[string]regexp.Regexp{}
var matchComplexityOnce sync.Once

// CheckPasswordComplexity return True if password is Complexity
func CheckPasswordComplexity(pwd string) bool {
	if len(setting.PasswordComplexity) > 0 {
		matchComplexityOnce.Do(func() {
			for key, val := range setting.PasswordComplexity {
				matchComplexity := regexp.MustCompile(val)
				matchComplexities[key] = *matchComplexity
			}
		})
		for _, val := range matchComplexities {
			if !val.MatchString(pwd) {
				return false
			}
		}
	}
	return true
}

// Generate  a random password
func Generate(n int) string {
	rand.Seed(time.Now().UnixNano())
	var dict = map[int]string{
		0: "abcdefghijklmnopqrstuvwxyz",
		1: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		2: "0123456789",
		3: "_-",
	}
	buffer := make([]byte, n)
	for {
		for j := 0; j < n; j++ {
			t := rand.Intn(4)
			index := rand.Intn(len(dict[t]))
			tmp := dict[t]
			buffer[j] = tmp[index]
		}
		for i := len(buffer) - 1; i > 0; i-- {
			j := rand.Intn(i + 1)
			buffer[i], buffer[j] = buffer[j], buffer[i]
		}
		if CheckPasswordComplexity(string(buffer)) {
			return string(buffer)
		}
	}
}
