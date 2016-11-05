package bindata

import (
	"testing"
	"io/ioutil"
)

func Test_resolveName(t *testing.T) {

	files := []string {
		"modules/bindata/dynamic_test.go",
		"conf/app.ini",
		"templates/home.tmpl",
	}

	for _, c := range files {
		name, _ := resolveName(c)
		_, err := ioutil.ReadFile(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

}
