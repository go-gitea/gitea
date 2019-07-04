// +build tools

package tools

import (
	_ "github.com/client9/misspell/cmd/misspell"
	_ "github.com/go-swagger/go-swagger/cmd/swagger"
	_ "github.com/jteeuwen/go-bindata/go-bindata"
	_ "github.com/kisielk/errcheck"
	_ "github.com/mgechev/revive"
	_ "github.com/ulikunitz/xz/cmd/gxz"
	_ "github.com/wadey/gocovmerge"
	_ "src.techknowlogick.com/xgo"
)
