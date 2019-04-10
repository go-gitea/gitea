//+build tools

// tools is a shim package to cause go-bindata to register as a dependency
package tools

import (
	_ "github.com/jteeuwen/go-bindata/go-bindata"
)
